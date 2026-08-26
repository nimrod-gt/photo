package app

import (
	"context"
	"errors"
	"log"
	"maps"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"fyne.io/fyne/v2"

	"photo/internal/core/imaging"
	"photo/internal/core/model"
	"photo/internal/core/tags"
)

// The tagger is held as what a run asks of it, so a test can hand the runner a
// generator that answers on demand instead of spawning a claude process.
type tagGenerator interface {
	Generate(ctx context.Context, req tags.Request) (model.Tags, error)
}

// A run outlives the dialog that started it, so the two are kept apart: the
// dialog is a listener the run may lose, not the owner of the work. What the
// run always does is write the tags where they belong; what it only does while
// a dialog listens is fill its fields.
//
// The registry is locked rather than left to the UI goroutine: a run reports
// itself through fyne.Do, which hops to that goroutine in the app but runs
// where it stands under the test driver, and a rule only one of the two obeys
// is no rule at all.
type tagRunner struct {
	app     *Application
	mu      sync.Mutex
	running sync.WaitGroup
	runs    map[string]*tagRun
}

// A run is over for the registry once the generation answered (landed), and
// over for whoever waits on it only once its last file write is done
// (released). Copy and delete wait for the second one: what would hurt them is
// a sidecar written after they touched the files, not a claude process.
type tagRun struct {
	photo     model.Photo
	taken     time.Time
	stop      context.CancelFunc
	session   *tagsSession
	typed     model.Tags
	done      chan struct{}
	cancelled bool
	landed    bool
	released  bool
}

const runShutdownWait = 2 * time.Second

func newTagRunner(app *Application) *tagRunner {
	return &tagRunner{app: app, runs: make(map[string]*tagRun)}
}

// attach hands a run that is still going to the dialog just opened over it, so
// pressing T on that photo again picks the run up instead of paying for a
// second one.
func (r *tagRunner) attach(session *tagsSession) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	run, ok := r.runs[session.photo.ImagePath]
	if !ok || run.landed || run.cancelled {
		return false
	}
	run.session = session
	return true
}

func (r *tagRunner) pending(path string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.runs[path]
	return ok
}

// takeOver hands the fields of a dialog closing over a running generation to
// the run, which writes the sidecar itself when it lands: one writer instead of
// two racing for the same file with different tags. The run replaces them with
// what it generated and falls back to them when it generates nothing, so
// nothing typed is lost either way.
//
// A run that was cancelled writes nothing, and one that already landed is
// writing what it has right now, so both refuse and leave the save where it
// was - which is why pending, true for all three, cannot answer this.
func (r *tagRunner) takeOver(path string, tags model.Tags) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	run, ok := r.runs[path]
	if !ok || run.landed || run.cancelled {
		return false
	}
	run.typed = tags
	return true
}

func (r *tagRunner) typedTags(run *tagRun) model.Tags {
	r.mu.Lock()
	defer r.mu.Unlock()

	return run.typed
}

// wait returns once the run for this photo has let go of its files, or at once
// when there is none. It belongs on a worker goroutine: most runs are released
// from inside fyne.Do, so waiting on the UI goroutine would hold up the very
// callback that ends the wait.
func (r *tagRunner) wait(ctx context.Context, path string) {
	r.mu.Lock()
	run, ok := r.runs[path]
	r.mu.Unlock()
	if !ok {
		return
	}

	select {
	case <-run.done:
	case <-ctx.Done():
	}
}

// The photo is the key, so a second run for one photo cannot exist: the dialog
// disables Generate while its own run goes, and a dialog reopened over a
// running one attaches to it instead of starting another.
func (r *tagRunner) start(session *tagsSession, req tags.Request) {
	path := session.photo.ImagePath
	ctx, cancel := context.WithCancel(context.Background())
	run := &tagRun{
		photo:   session.photo,
		taken:   session.taken,
		stop:    cancel,
		session: session,
		done:    make(chan struct{}),
	}

	r.mu.Lock()
	r.runs[path] = run
	r.mu.Unlock()

	r.running.Add(1)
	go func() {
		defer cancel()
		generated, err := r.app.tagger.Generate(ctx, req)
		// Only the claude process is waited for on the way out: the report
		// below needs the UI goroutine, which is gone by then.
		r.running.Done()
		fyne.Do(func() {
			r.finish(run, req.ClaudePath, generated, err)
		})
	}()
}

// A cancelled run says nothing and writes nothing: the user who pressed Cancel
// knows what happened, and tags that landed in the same moment are not theirs
// to keep - which is why the flag is asked about and not only the error.
func (r *tagRunner) finish(run *tagRun, claudePath string, generated model.Tags, err error) {
	cancelled, session := r.unregister(run)
	if cancelled || errors.Is(err, context.Canceled) {
		r.release(run)
		return
	}
	if err != nil {
		log.Println("Failed to generate tags:", err)
	} else {
		// Only a path that produced tags is remembered, and an empty one
		// clears the preference: a stored path short-circuits the search for
		// the binary, so a typo saved eagerly would disable it for good.
		r.app.fyneApp.Preferences().SetString("claudePath", claudePath)
	}

	if session != nil && r.app.dialogs.isCurrent(session.dialog) {
		// What the dialog writes from here on is the user's save, not the
		// run's, and is not tracked: a dialog on screen blocks the copy and
		// delete shortcuts, and the window between closing it and its own
		// write landing is the one it already had before any of this.
		r.release(run)
		session.runFinished(generated, err)
		return
	}
	r.finishDetached(run, generated, err)
}

// unregister marks the generation as answered and reports what the run was told
// while it went: whether it was cancelled, and which dialog - if any - is still
// waiting for it. The entry stays in the registry until release, so a wait
// started meanwhile still finds something to wait on; a dialog reopened in that
// window is refused by attach and reads the file instead.
func (r *tagRunner) unregister(run *tagRun) (cancelled bool, session *tagsSession) {
	r.mu.Lock()
	defer r.mu.Unlock()

	run.landed = true
	return run.cancelled, run.session
}

func (r *tagRunner) dropped(run *tagRun) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return run.cancelled
}

// release is the one place a run ends. It drops the entry - unless a later run
// has taken the key meanwhile, which is then that run's to hold - and frees
// whoever waits on the photo, so a copy or a delete only sees files no run will
// touch again.
func (r *tagRunner) release(run *tagRun) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if run.released {
		return
	}
	run.released = true
	path := run.photo.ImagePath
	if r.runs[path] == run {
		delete(r.runs, path)
	}
	close(run.done)
}

// Nothing is on screen for this photo any more, so the run reports itself: what
// the dialog would have shown goes to the notifier, and what it would have
// saved is saved here.
//
// The cache and the overlay are only told once the sidecar holds the tags. A
// cached entry means "this file has them", which is what stops the next dialog
// from writing them again, so a failed write must leave nothing behind.
func (r *tagRunner) finishDetached(run *tagRun, generated model.Tags, err error) {
	if err != nil {
		r.app.showError("Failed to generate tags for "+run.photo.Name, err)
		// A generation that brought nothing still owes the sidecar the fields
		// the dialog handed over when it closed, which are all the photo has.
		r.saveTyped(run)
		return
	}

	taken := run.dateAt(r.app)
	if !run.photo.HasRAW() {
		r.report(run, generated, taken)
		r.release(run)
		return
	}
	r.saveSidecar(run, generated, taken, true)
}

func (r *tagRunner) saveTyped(run *tagRun) {
	typed := r.typedTags(run)
	if !run.photo.HasRAW() || nothingToWrite(typed) {
		r.release(run)
		return
	}
	r.saveSidecar(run, typed, run.dateAt(r.app), false)
}

// A delete cancels the run it is about to remove the files of, and that can
// land while the write below is going. What it says about the photo is then
// about a file that is on its way out: the cache would keep tags no file holds,
// and the notification would name a photo the user just deleted.
func (r *tagRunner) saveSidecar(run *tagRun, written model.Tags, taken time.Time, generated bool) {
	path := model.SidecarPath(run.photo.RAWPath)
	go func() {
		err := imaging.WriteSidecar(path, written)
		// Freed by the file being on disk, not by the UI goroutine being free:
		// a copy waiting on this run needs the sidecar, not the notification.
		r.release(run)
		fyne.Do(func() {
			if r.dropped(run) {
				return
			}
			if err != nil {
				r.app.showError("Failed to save tags to "+filepath.Base(path), err)
				return
			}
			if generated {
				r.report(run, written, taken)
				return
			}
			r.store(run, written, taken)
			r.app.mainWindow.ShowNotification("Tags saved to " + filepath.Base(path))
		})
	}()
}

func (r *tagRunner) report(run *tagRun, generated model.Tags, taken time.Time) {
	r.store(run, generated, taken)
	r.app.mainWindow.ShowNotification("Tags generated for " + run.photo.Name)
}

func (r *tagRunner) store(run *tagRun, written model.Tags, taken time.Time) {
	r.app.imageProvider.StoreStockInfo(run.photo.ImagePath, imaging.StockInfo{Tags: written, Taken: taken})
	r.app.setTagsIfCurrent(run.photo.ImagePath, written)
}

// The date the run started with is the one the cache held then, which for a
// photo whose image had not been read yet is none at all. A read that landed
// while the run went knows the real one, and storing the older answer over it
// would leave the next dialog with an empty date.
func (run *tagRun) dateAt(app *Application) time.Time {
	if taken, ok := app.imageProvider.PeekStockDate(run.photo.ImagePath); ok {
		return taken
	}
	return run.taken
}

// background leaves the run going and takes its listener away, so it reports
// itself when it lands.
func (r *tagRunner) background(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if run, ok := r.runs[path]; ok {
		run.session = nil
	}
}

// A cancelled run keeps its entry until it lets go of the files, so a delete
// can wait for it; attach refuses it meanwhile, which is what keeps a dialog
// reopened on the photo from sitting at "Generating" for good.
func (r *tagRunner) cancel(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if run, ok := r.runs[path]; ok {
		run.cancelled = true
		run.stop()
	}
}

// awaitTags holds the caller until nothing is going to write for this photo any
// more, so a copy takes the sidecar a run is about to write with it instead of
// leaving the tags behind. The wait is announced, because a copy that stops for
// a minute with nothing on screen reads as a hang.
//
// Like stopTags, it belongs on a worker goroutine - see tagRunner.wait.
func (a *Application) awaitTags(ctx context.Context, photo model.Photo) {
	if !a.tagRuns.pending(photo.ImagePath) {
		return
	}
	fyne.Do(func() {
		a.mainWindow.ShowNotification("Waiting for tags of " + photo.Name)
	})
	a.tagRuns.wait(ctx, photo.ImagePath)
}

// A delete kills the run first: its tags describe a photo that is about to be
// gone, and waiting out a three-minute generation for them helps nobody. The
// wait still follows, because a kill does not stop a sidecar write that already
// started, and what a delete needs is the files free rather than the run over.
func (a *Application) stopTags(photo model.Photo) {
	a.tagRuns.cancel(photo.ImagePath)
	a.tagRuns.wait(context.Background(), photo.ImagePath)
}

// stopAll waits because the claude processes are killed by the goroutine
// context cancellation starts, and the app is on its way out: without the wait
// the window can be gone before that kill lands, leaving a claude process
// behind for the rest of its three minutes.
func (r *tagRunner) stopAll() {
	r.mu.Lock()
	for _, run := range r.runs {
		run.cancelled = true
		run.stop()
	}
	r.mu.Unlock()

	done := make(chan struct{})
	go func() {
		r.running.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(runShutdownWait):
		log.Println("Gave up waiting for tag generation to stop")
	}

	// A run is released where it reports itself, which needs a UI goroutine
	// that is going away, so these runs would never let go on their own and
	// anything still waiting on one would wait for the process to die.
	r.mu.Lock()
	left := slices.Collect(maps.Values(r.runs))
	r.mu.Unlock()
	for _, run := range left {
		r.release(run)
	}
}

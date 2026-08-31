package app

import (
	"context"
	"errors"
	"log"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"

	"photo/internal/core/model"
	"photo/internal/core/tags"
	"photo/internal/gui/ui"
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
	photo   model.Photo
	started time.Time
	taken   time.Time
	stop    context.CancelFunc
	session *tagsSession
	typed   model.Tags
	// typedComplete says the dialog that handed the fields over knew what the
	// files held when it did, so what it left empty was emptied rather than
	// never read - see completed.
	typedComplete bool
	done          chan struct{}
	cancelled     bool
	landed        bool
	writing       bool
	released      bool
}

const runShutdownWait = 2 * time.Second

func newTagRunner(app *Application) *tagRunner {
	return &tagRunner{app: app, runs: make(map[string]*tagRun)}
}

// attach hands a run that is still going to the dialog just opened over it, so
// pressing T on that photo again picks the run up instead of paying for a
// second one. The fields an earlier dialog left with the run come back with it:
// the run answers to a dialog again and writes nothing of its own, so what was
// typed would be gone if it stayed here.
func (r *tagRunner) attach(session *tagsSession) (model.Tags, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	run, ok := r.runs[session.photo.ImagePath]
	if !ok || run.landed || run.cancelled {
		return model.Tags{}, false
	}
	run.session = session
	return run.typed, true
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
func (r *tagRunner) takeOver(path string, tags model.Tags, complete bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	run, ok := r.runs[path]
	if !ok || run.landed || run.cancelled {
		return false
	}
	run.typed = tags
	run.typedComplete = complete
	return true
}

func (r *tagRunner) typedTags(run *tagRun) (typed model.Tags, complete bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return run.typed, run.typedComplete
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
		started: time.Now(),
		taken:   session.taken,
		stop:    cancel,
		session: session,
		done:    make(chan struct{}),
	}

	r.mu.Lock()
	r.runs[path] = run
	r.mu.Unlock()
	r.reportRuns()

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
	r.reportRuns()
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
		r.saveTyped(run, err)
		return
	}

	// A generation answers with the whole set, so it goes in where the sidecar
	// stood rather than being added to it.
	r.saveFiles(run, generated, run.dateAt(r.app), true, func(_, note string) {
		if len(note) != 0 {
			r.app.notifier.ShowWarning("Tags generated for " + run.photo.Name + " - " + note)
			return
		}
		r.app.notifier.ShowNotification("Tags generated for " + run.photo.Name)
	})
}

// A generation that brought nothing still owes the files the fields the dialog
// handed over when it closed, which are all the photo has. The failure is what
// the user is told, and only once that write is over: the notifier holds one
// message at a time, so a save nobody asked for announcing itself would push
// the error that explains it off the screen.
//
// Where the fields were kept is named by the save itself: with the autosaves
// off they reached no file at all and only the cache and the overlay have them.
func (r *tagRunner) saveTyped(run *tagRun, failure error) {
	typed, complete := r.typedTags(run)
	if nothingToWrite(typed) {
		r.release(run)
		r.app.showError("Failed to generate tags for "+run.photo.Name, failure)
		return
	}
	r.saveFiles(run, typed, run.dateAt(r.app), complete, func(target, _ string) {
		kept := ", kept what was typed"
		if len(target) != 0 {
			kept += " in " + target
		}
		r.app.showError("Failed to generate tags for "+run.photo.Name+kept, failure)
	})
}

// A delete cancels the run it is about to remove the files of, and that can
// land while the write below is going. What it says about the photo is then
// about a file that is on its way out: the cache would keep tags no file holds,
// and the notification would name a photo the user just deleted.
//
// The settings decide what is written, and with both of them off the run has
// nothing but the cache, the overlay and its notification to offer: the tags
// are shown and wait for the Save button of a dialog reopened on the photo.
func (r *tagRunner) saveFiles(run *tagRun, written model.Tags, taken time.Time, complete bool, saved func(target, note string)) {
	plan := r.app.autoWrite().forPhoto(run.photo, written)
	if plan.none() {
		r.release(run)
		if r.dropped(run) {
			return
		}
		r.app.storeStock(run.photo.ImagePath, written, taken, false)
		saved("", "")
		return
	}

	target := writeTarget(run.photo, plan)
	r.writeStarted(run)
	go func() {
		written, note, err := r.app.writeTagFiles(run.photo, written, plan, complete)
		// Freed by the file being on disk, not by the UI goroutine being free:
		// a copy waiting on this run needs the sidecar, not the notification.
		r.release(run)
		fyne.Do(func() {
			if r.dropped(run) {
				return
			}
			if err != nil {
				r.app.showError("Failed to save tags to "+target, err)
				return
			}
			r.app.storeStock(run.photo.ImagePath, written, taken, plan.sidecar)
			saved(target, note)
		})
	}()
}

// The run is the sidecar's writer from here on, which is what keeps the exit
// flush from putting the fields it was handed on top of what it found.
func (r *tagRunner) writeStarted(run *tagRun) {
	r.mu.Lock()
	defer r.mu.Unlock()

	run.writing = true
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
	if run, ok := r.runs[path]; ok {
		run.cancelled = true
		run.stop()
	}
	r.mu.Unlock()

	r.reportRuns()
}

// What the corner shows is every generation still in flight: a run that landed
// is only writing its file, and a cancelled one is only letting go of it, so
// neither is worth a plate any more. The order is the order they started in,
// because the registry is a map and would hand them over in a different one
// every time.
func (r *tagRunner) live() []ui.RunItem {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]ui.RunItem, 0, len(r.runs))
	for _, run := range r.runs {
		if run.landed || run.cancelled {
			continue
		}
		items = append(items, ui.RunItem{Name: run.photo.Name, Since: run.started})
	}
	slices.SortFunc(items, func(a, b ui.RunItem) int {
		if order := a.Since.Compare(b.Since); order != 0 {
			return order
		}
		return strings.Compare(a.Name, b.Name)
	})
	return items
}

// The list is taken on the other side of the hop rather than here: a report from
// a worker goroutine - a delete stopping a run - would otherwise carry what the
// registry held before it queued, and land behind a newer one that already
// showed a run it knew nothing about.
//
// stopAll is no caller of this: the window is on its way out there, and what it
// kills has nowhere left to be shown.
func (r *tagRunner) reportRuns() {
	fyne.Do(func() {
		r.app.runBar.SetRuns(r.live())
	})
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
		a.notifier.ShowNotification("Waiting for tags of " + photo.Name)
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
	stopped := slices.Collect(maps.Values(r.runs))
	for _, run := range stopped {
		run.cancelled = true
		run.stop()
	}
	r.mu.Unlock()

	// Straight to the window rather than through reportRuns: the loop that would
	// carry the hop is over, and a corner left with a plate on it keeps arming a
	// tick for a run that was just killed. It comes after the kill, so a run that
	// answers in the meantime finds itself cancelled and puts nothing back.
	r.app.runBar.SetRuns(nil)

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

	// The runs are taken from before the kill rather than from the registry
	// now: a run that answered meanwhile dropped itself out of it, and the
	// fields it was handed are still owed a file.
	//
	// A run is released where it reports itself, which needs a UI goroutine
	// that is going away, so runs still in the registry would never let go on
	// their own and anything waiting on one would wait for the process to die.
	for _, run := range stopped {
		r.flushTyped(run)
		r.release(run)
	}
}

// A dialog that closed over a run left its fields with it, and the run was
// going to write them when it landed - which it never will now. The write is
// done here instead, on the way out and on this goroutine, because what would
// hold a written file is gone with the UI.
//
// A run that started its own write is left alone: putting the older fields on
// top of what it found is the very race the hand-over exists to stop.
// What is written is what the settings would have let the run write, so a file
// left to the Save button is left to it here too: those fields were never on
// their way to it.
func (r *tagRunner) flushTyped(run *tagRun) {
	typed, complete, ok := r.unwrittenTyped(run)
	if !ok || nothingToWrite(typed) {
		return
	}
	plan := r.app.autoWrite().forPhoto(run.photo, typed)
	if plan.none() {
		return
	}
	if _, _, err := r.app.writeTagFiles(run.photo, typed, plan, complete); err != nil {
		log.Println("Failed to save tags on the way out:", err)
	}
}

func (r *tagRunner) unwrittenTyped(run *tagRun) (typed model.Tags, complete, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return run.typed, run.typedComplete, !run.writing
}

package app

import (
	"context"
	"errors"
	"log"
	"path/filepath"
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

type tagRun struct {
	photo     model.Photo
	taken     time.Time
	stop      context.CancelFunc
	session   *tagsSession
	cancelled bool
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
	if !ok {
		return false
	}
	run.session = session
	return true
}

// The photo is the key, so a second run for one photo cannot exist: the dialog
// disables Generate while its own run goes, and a dialog reopened over a
// running one attaches to it instead of starting another.
func (r *tagRunner) start(session *tagsSession, req tags.Request) {
	path := session.photo.ImagePath
	ctx, cancel := context.WithCancel(context.Background())
	run := &tagRun{photo: session.photo, taken: session.taken, stop: cancel, session: session}

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
			r.finish(run, req, generated, err)
		})
	}()
}

// A cancelled run says nothing and writes nothing: the user who pressed Cancel
// knows what happened, and tags that landed in the same moment are not theirs
// to keep - which is why the flag is asked about and not only the error.
func (r *tagRunner) finish(run *tagRun, req tags.Request, generated model.Tags, err error) {
	cancelled, session := r.unregister(run)
	if cancelled || errors.Is(err, context.Canceled) {
		return
	}
	if err != nil {
		log.Println("Failed to generate tags:", err)
	} else {
		// Only a path that produced tags is remembered, and an empty one
		// clears the preference: a stored path short-circuits the search for
		// the binary, so a typo saved eagerly would disable it for good.
		r.app.fyneApp.Preferences().SetString("claudePath", req.ClaudePath)
	}

	if session != nil && r.app.dialogs.isCurrent(session.dialog) {
		session.runFinished(generated, err)
		return
	}
	r.finishDetached(run, generated, err)
}

// unregister takes the run out of the registry and answers with what it was
// told while it ran: whether it was cancelled, and which dialog - if any - is
// still waiting for it. A run cancel already removed is not removed twice, and
// a key some other run has taken meanwhile is left to that run.
func (r *tagRunner) unregister(run *tagRun) (cancelled bool, session *tagsSession) {
	r.mu.Lock()
	defer r.mu.Unlock()

	path := run.photo.ImagePath
	if r.runs[path] == run {
		delete(r.runs, path)
	}
	return run.cancelled, run.session
}

// Nothing is on screen for this photo any more, so the run reports itself: what
// the dialog would have shown goes to the notifier, and what it would have
// saved is saved here.
func (r *tagRunner) finishDetached(run *tagRun, generated model.Tags, err error) {
	if err != nil {
		r.app.showError("Failed to generate tags for "+run.photo.Name, err)
		return
	}

	r.app.imageProvider.StoreStockInfo(run.photo.ImagePath, imaging.StockInfo{Tags: generated, Taken: run.dateAt(r.app)})
	r.app.setTagsIfCurrent(run.photo.ImagePath, generated)
	if !run.photo.HasRAW() {
		r.app.mainWindow.ShowNotification("Tags generated for " + run.photo.Name)
		return
	}

	path := model.SidecarPath(run.photo.RAWPath)
	go func() {
		err := imaging.WriteSidecar(path, generated)
		fyne.Do(func() {
			if err != nil {
				// The tags are in the cache either way, so the dialog still
				// offers them; only the file missed them.
				r.app.imageProvider.Forget(run.photo.ImagePath)
				r.app.showError("Failed to save tags to "+filepath.Base(path), err)
				return
			}
			r.app.mainWindow.ShowNotification("Tags generated for " + run.photo.Name)
		})
	}()
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

// visible answers with the session of the run the user is looking at - the one
// the Background key means. At most one dialog is open, so at most one run can
// be that one.
func (r *tagRunner) visible() (*tagsSession, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, run := range r.runs {
		if run.session != nil && r.app.dialogs.isCurrent(run.session.dialog) {
			return run.session, true
		}
	}
	return nil, false
}

// The run leaves the registry before it stops: a dialog reopened on the photo
// meanwhile would otherwise attach to a run that reports nothing and sit at
// "Generating" for good.
func (r *tagRunner) cancel(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if run, ok := r.runs[path]; ok {
		run.cancelled = true
		run.stop()
		delete(r.runs, path)
	}
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
}

package app

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"photo/internal/core/library"
)

type dialogKind int

const (
	dialogNone dialogKind = iota
	dialogDelete
	dialogCopy
	dialogHelp
	dialogDeleteAll
	dialogCopyAll
	dialogUnselectAll
	dialogTags
	dialogSettings
)

type hideable interface {
	Hide()
}

type dialogManager struct {
	kind      dialogKind
	dialog    hideable
	onCancel  func()
	ownsClose bool
}

func (m *dialogManager) open(kind dialogKind, d hideable, onCancel func()) {
	m.kind = kind
	m.dialog = d
	m.onCancel = onCancel
	m.ownsClose = false
}

// openSelfClosing hands cancelling over to onCancel whole: the dialog decides
// when it unregisters and hides, so Escape only signals it. A running bulk
// copy keeps its window up until the copy goroutine stops.
func (m *dialogManager) openSelfClosing(kind dialogKind, d hideable, onCancel func()) {
	m.open(kind, d, onCancel)
	m.ownsClose = true
}

func (m *dialogManager) closed() {
	m.kind = dialogNone
	m.dialog = nil
	m.onCancel = nil
	m.ownsClose = false
}

func (m *dialogManager) isOpen(kind dialogKind) bool {
	return m.kind == kind
}

func (m *dialogManager) isCurrent(d hideable) bool {
	return m.dialog == d
}

func (m *dialogManager) anyOpen() bool {
	return m.kind != dialogNone
}

func (m *dialogManager) confirm() {
	if c, ok := m.dialog.(interface{ Confirm() }); ok {
		c.Confirm()
	}
}

func (m *dialogManager) cancel() {
	if m.kind == dialogNone {
		return
	}
	if m.ownsClose {
		if m.onCancel != nil {
			m.onCancel()
		}
		return
	}
	d := m.dialog
	onCancel := m.onCancel
	m.closed()
	if onCancel != nil {
		onCancel()
	}
	if d != nil {
		d.Hide()
	}
}

// A repeated press of the key that opened a confirm dialog answers it, so the
// dialog's own kind confirms instead of blocking.
func (a *Application) dialogBlocked(kind dialogKind) bool {
	if a.gridMode {
		return true
	}
	if a.dialogs.isOpen(kind) {
		a.dialogs.confirm()
		return true
	}
	return a.dialogs.anyOpen()
}

func (a *Application) showConfirm(kind dialogKind, title, confirmText, cancelText string, content fyne.CanvasObject, onConfirm func()) {
	confirmDialog := dialog.NewCustomConfirm(title, confirmText, cancelText,
		content,
		func(confirmed bool) {
			a.dialogs.closed()
			if !confirmed {
				return
			}
			onConfirm()
		},
		a.mainWindow.Window(),
	)
	a.dialogs.open(kind, confirmDialog, nil)
	confirmDialog.Show()
}

func (a *Application) showErrorAsync(msg string, err error) {
	fyne.Do(func() {
		a.showError(msg, err)
	})
}

func (a *Application) copyPreferences() (string, library.CopyMode) {
	prefs := a.fyneApp.Preferences()
	return prefs.String("copyDestination"),
		library.CopyMode(prefs.IntWithFallback("copyMode", int(library.CopyWithRAW)))
}

func (a *Application) saveCopyPreferences(dest string, mode library.CopyMode) {
	prefs := a.fyneApp.Preferences()
	prefs.SetString("copyDestination", dest)
	prefs.SetInt("copyMode", int(mode))
}

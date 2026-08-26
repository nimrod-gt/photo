package ui

import (
	"fyne.io/fyne/v2"
	"github.com/go-gl/glfw/v3.4/glfw"
)

type ShortcutCallbacks struct {
	OnFavorite       func()
	OnRed            func()
	OnGreen          func()
	OnBlue           func()
	OnDelete         func()
	OnCopy           func()
	OnCancel         func()
	OnNext           func()
	OnPrevious       func()
	OnSort           func()
	OnFilterRed      func()
	OnFilterGreen    func()
	OnFilterBlue     func()
	OnFilterFavorite func()
	OnHelp           func()
	OnCopyClipboard  func()
	OnToggleGrid     func()
	OnZoomReset      func()
	OnZoomIn         func()
	OnZoomOut        func()
	OnTags           func()
	OnToggleTags     func()
}

// A shortcut is bound to the place of the key on the keyboard rather than to
// the letter a layout prints on it. GLFW knows those places only once it is
// initialised, which happens behind a real window, so the lookup is handed over
// by whoever wired the canvas up; a matcher without one reads the name of the
// key instead, and that is what keeps N and B apart under test.
type KeyMatcher struct {
	scancode func(glfw.Key) int
}

func (m KeyMatcher) Matches(ev *fyne.KeyEvent, key glfw.Key, name fyne.KeyName) bool {
	if m.scancode == nil {
		return ev.Name == name
	}
	return ev.Physical.ScanCode == m.scancode(key)
}

func SetupShortcuts(canvas fyne.Canvas, callbacks ShortcutCallbacks) KeyMatcher {
	scanActions := map[int]func(){
		glfw.GetKeyScancode(glfw.KeyF): callbacks.OnFavorite,
		glfw.GetKeyScancode(glfw.KeyR): callbacks.OnRed,
		glfw.GetKeyScancode(glfw.KeyG): callbacks.OnGreen,
		glfw.GetKeyScancode(glfw.KeyB): callbacks.OnBlue,
		glfw.GetKeyScancode(glfw.KeyD): callbacks.OnDelete,
		glfw.GetKeyScancode(glfw.KeyC): callbacks.OnCopy,
		glfw.GetKeyScancode(glfw.KeyN): callbacks.OnCancel,
		glfw.GetKeyScancode(glfw.KeyS): callbacks.OnSort,
		glfw.GetKeyScancode(glfw.Key1): callbacks.OnFilterFavorite,
		glfw.GetKeyScancode(glfw.Key2): callbacks.OnFilterRed,
		glfw.GetKeyScancode(glfw.Key3): callbacks.OnFilterGreen,
		glfw.GetKeyScancode(glfw.Key4): callbacks.OnFilterBlue,
		glfw.GetKeyScancode(glfw.KeyH): callbacks.OnHelp,
		glfw.GetKeyScancode(glfw.KeyY): callbacks.OnCopyClipboard,
		glfw.GetKeyScancode(glfw.KeyL): callbacks.OnToggleGrid,
		glfw.GetKeyScancode(glfw.KeyZ): callbacks.OnZoomReset,
		glfw.GetKeyScancode(glfw.KeyT): callbacks.OnTags,
		glfw.GetKeyScancode(glfw.KeyI): callbacks.OnToggleTags,
	}

	keyActions := map[fyne.KeyName]func(){
		fyne.KeyPlus:   callbacks.OnZoomIn,
		fyne.KeyEqual:  callbacks.OnZoomIn,
		fyne.KeyMinus:  callbacks.OnZoomOut,
		fyne.KeyEscape: callbacks.OnCancel,
	}

	canvas.SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if action, ok := scanActions[ev.Physical.ScanCode]; ok {
			action()
			return
		}
		if action, ok := keyActions[ev.Name]; ok {
			action()
			return
		}
		switch ev.Name {
		case fyne.KeyRight, fyne.KeyDown:
			callbacks.OnNext()
		case fyne.KeyLeft, fyne.KeyUp:
			callbacks.OnPrevious()
		}
	})

	return KeyMatcher{scancode: glfw.GetKeyScancode}
}

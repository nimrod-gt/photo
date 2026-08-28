package ui

import (
	"fyne.io/fyne/v2"
	"github.com/go-gl/glfw/v3.4/glfw"
)

// Taken as a variable so a test can stand in for GLFW, which reports no scan
// code at all until a window has been opened.
var keyScanCode = glfw.GetKeyScancode

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
	OnSettings       func()
}

// A shortcut is bound to the place of the key on the keyboard rather than to
// the letter a layout prints on it, so the keys mean the same thing whatever
// layout the user types in.
func SetupShortcuts(canvas fyne.Canvas, callbacks ShortcutCallbacks) {
	scanActions := map[int]func(){
		keyScanCode(glfw.KeyF): callbacks.OnFavorite,
		keyScanCode(glfw.KeyR): callbacks.OnRed,
		keyScanCode(glfw.KeyG): callbacks.OnGreen,
		keyScanCode(glfw.KeyB): callbacks.OnBlue,
		keyScanCode(glfw.KeyD): callbacks.OnDelete,
		keyScanCode(glfw.KeyC): callbacks.OnCopy,
		keyScanCode(glfw.KeyN): callbacks.OnCancel,
		keyScanCode(glfw.KeyS): callbacks.OnSort,
		keyScanCode(glfw.Key1): callbacks.OnFilterFavorite,
		keyScanCode(glfw.Key2): callbacks.OnFilterRed,
		keyScanCode(glfw.Key3): callbacks.OnFilterGreen,
		keyScanCode(glfw.Key4): callbacks.OnFilterBlue,
		keyScanCode(glfw.KeyH): callbacks.OnHelp,
		keyScanCode(glfw.KeyY): callbacks.OnCopyClipboard,
		keyScanCode(glfw.KeyL): callbacks.OnToggleGrid,
		keyScanCode(glfw.KeyZ): callbacks.OnZoomReset,
		keyScanCode(glfw.KeyT): callbacks.OnTags,
		keyScanCode(glfw.KeyI): callbacks.OnSettings,
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
}

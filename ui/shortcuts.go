package ui

import (
	"fyne.io/fyne/v2"
	"github.com/go-gl/glfw/v3.3/glfw"
)

type ShortcutCallbacks struct {
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
}

func SetupShortcuts(canvas fyne.Canvas, callbacks ShortcutCallbacks) {
	scanActions := map[int]func(){
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
	}

	keyActions := map[fyne.KeyName]func(){
		fyne.KeyPlus:  callbacks.OnZoomIn,
		fyne.KeyEqual: callbacks.OnZoomIn,
		fyne.KeyMinus: callbacks.OnZoomOut,
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

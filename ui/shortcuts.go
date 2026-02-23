package ui

import (
	"fyne.io/fyne/v2"
	"github.com/go-gl/glfw/v3.3/glfw"
)

type ShortcutCallbacks struct {
	OnFavorite func()
	OnRed      func()
	OnGreen    func()
	OnBlue     func()
	OnDelete   func()
	OnNext     func()
	OnPrevious func()
}

func SetupShortcuts(canvas fyne.Canvas, callbacks ShortcutCallbacks) {
	scanActions := map[int]func(){
		glfw.GetKeyScancode(glfw.KeyF): callbacks.OnFavorite,
		glfw.GetKeyScancode(glfw.KeyR): callbacks.OnRed,
		glfw.GetKeyScancode(glfw.KeyG): callbacks.OnGreen,
		glfw.GetKeyScancode(glfw.KeyB): callbacks.OnBlue,
		glfw.GetKeyScancode(glfw.KeyD): callbacks.OnDelete,
	}

	canvas.SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if action, ok := scanActions[ev.Physical.ScanCode]; ok {
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

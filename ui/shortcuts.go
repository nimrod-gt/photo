package ui

import "fyne.io/fyne/v2"

type ShortcutCallbacks struct {
	OnRed      func()
	OnGreen    func()
	OnBlue     func()
	OnDelete   func()
	OnNext     func()
	OnPrevious func()
}

func SetupShortcuts(canvas fyne.Canvas, callbacks ShortcutCallbacks) {
	canvas.SetOnTypedRune(func(r rune) {
		switch r {
		case 'r', 'R':
			if callbacks.OnRed != nil {
				callbacks.OnRed()
			}
		case 'g', 'G':
			if callbacks.OnGreen != nil {
				callbacks.OnGreen()
			}
		case 'b', 'B':
			if callbacks.OnBlue != nil {
				callbacks.OnBlue()
			}
		case 'd', 'D':
			if callbacks.OnDelete != nil {
				callbacks.OnDelete()
			}
		}
	})

	canvas.SetOnTypedKey(func(ev *fyne.KeyEvent) {
		switch ev.Name {
		case fyne.KeyRight, fyne.KeyDown:
			if callbacks.OnNext != nil {
				callbacks.OnNext()
			}
		case fyne.KeyLeft, fyne.KeyUp:
			if callbacks.OnPrevious != nil {
				callbacks.OnPrevious()
			}
		}
	})
}

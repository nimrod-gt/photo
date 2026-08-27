//go:build darwin

package keyname

// Ctrl is the Control symbol, not the Command one: the chords that answer to it
// take Command as well, but the name the keyboard carries is Control.
const (
	Shift = "⇧"
	Ctrl  = "⌃"
	Alt   = "⌥"
)

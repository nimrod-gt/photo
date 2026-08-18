package app

type dialogKind int

const (
	dialogNone dialogKind = iota
	dialogDelete
	dialogCopy
	dialogHelp
	dialogDeleteAll
	dialogCopyAll
	dialogUnselectAll
)

type hideable interface {
	Hide()
}

type dialogManager struct {
	kind     dialogKind
	dialog   hideable
	onCancel func()
}

func (m *dialogManager) open(kind dialogKind, d hideable, onCancel func()) {
	m.kind = kind
	m.dialog = d
	m.onCancel = onCancel
}

func (m *dialogManager) closed() {
	m.kind = dialogNone
	m.dialog = nil
	m.onCancel = nil
}

func (m *dialogManager) isOpen(kind dialogKind) bool {
	return m.kind == kind
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

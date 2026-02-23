package ui

import (
	"path/filepath"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"photo/service"
)

type DirTree struct {
	tree        *widget.Tree
	scanner     *service.Scanner
	mu          sync.RWMutex
	root        string
	hasChildren map[string]bool
}

func NewDirTree(scanner *service.Scanner, onSelected func(string)) *DirTree {
	dt := &DirTree{
		scanner:     scanner,
		hasChildren: make(map[string]bool),
	}

	dt.tree = widget.NewTree(
		func(uid widget.TreeNodeID) []widget.TreeNodeID {
			dt.mu.RLock()
			root := dt.root
			dt.mu.RUnlock()

			if len(uid) == 0 {
				if len(root) == 0 {
					return nil
				}
				return []string{root}
			}
			dirs, err := dt.scanner.ListDirectories(uid)
			if err != nil {
				return nil
			}

			dt.mu.Lock()
			for _, d := range dirs {
				if _, ok := dt.hasChildren[d]; !ok {
					children, _ := dt.scanner.ListDirectories(d)
					dt.hasChildren[d] = len(children) > 0
				}
			}
			dt.mu.Unlock()

			return dirs
		},
		func(uid widget.TreeNodeID) bool {
			dt.mu.RLock()
			v, ok := dt.hasChildren[uid]
			dt.mu.RUnlock()
			if ok {
				return v
			}
			return true
		},
		func(branch bool) fyne.CanvasObject {
			return widget.NewLabel("placeholder")
		},
		func(uid widget.TreeNodeID, branch bool, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(filepath.Base(uid))
		},
	)
	dt.tree.OnSelected = func(uid widget.TreeNodeID) {
		if onSelected != nil {
			onSelected(uid)
		}
	}

	return dt
}

func (dt *DirTree) SetRoot(root string) {
	dt.mu.Lock()
	dt.root = root
	dt.hasChildren = make(map[string]bool)
	dt.mu.Unlock()
	dt.tree.Refresh()
	dt.tree.OpenBranch(root)
}

func (dt *DirTree) Widget() *widget.Tree {
	return dt.tree
}

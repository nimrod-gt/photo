package ui

import (
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/library"
)

type DirTree struct {
	tree        *widget.Tree
	scanner     *library.Scanner
	mu          sync.RWMutex
	root        string
	hasChildren map[string]bool
}

func NewDirTree(scanner *library.Scanner, onSelected func(string)) *DirTree {
	dt := &DirTree{
		scanner:     scanner,
		hasChildren: make(map[string]bool),
	}

	dt.tree = widget.NewTree(
		dt.childUIDs,
		dt.isBranch,
		func(branch bool) fyne.CanvasObject {
			icon := canvas.NewImageFromResource(iconFolderClosed)
			icon.FillMode = canvas.ImageFillContain
			icon.SetMinSize(fyne.NewSize(16, 16))
			return container.NewHBox(icon, widget.NewLabel("placeholder"))
		},
		func(uid widget.TreeNodeID, branch bool, obj fyne.CanvasObject) {
			box := obj.(*fyne.Container)
			icon := box.Objects[0].(*canvas.Image)
			label := box.Objects[1].(*widget.Label)
			label.SetText(filepath.Base(uid))
			switch {
			case !branch:
				icon.Resource = iconFolderLeaf
			case dt.tree.IsBranchOpen(uid):
				icon.Resource = iconFolderOpen
			default:
				icon.Resource = iconFolderClosed
			}
			icon.Refresh()
		},
	)
	dt.tree.OnSelected = func(uid widget.TreeNodeID) {
		if dt.isBranch(uid) {
			if dt.tree.IsBranchOpen(uid) {
				dt.tree.CloseBranch(uid)
			} else {
				dt.tree.OpenBranch(uid)
			}
		}
		if onSelected != nil {
			onSelected(uid)
		}
		dt.tree.UnselectAll()
	}

	return dt
}

func (dt *DirTree) childUIDs(uid widget.TreeNodeID) []widget.TreeNodeID {
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
		children, _ := dt.scanner.ListDirectories(d)
		dt.hasChildren[d] = len(children) > 0
	}
	dt.mu.Unlock()

	return dirs
}

func (dt *DirTree) isBranch(uid widget.TreeNodeID) bool {
	dt.mu.RLock()
	v, ok := dt.hasChildren[uid]
	dt.mu.RUnlock()
	if ok {
		return v
	}
	return true
}

func (dt *DirTree) SetRoot(root string) {
	dt.mu.Lock()
	dt.root = root
	dt.hasChildren = make(map[string]bool)
	dt.mu.Unlock()
	dt.tree.Refresh()
	dt.tree.OpenBranch(root)
}

func (dt *DirTree) OpenPath(path string) {
	dt.mu.RLock()
	root := dt.root
	dt.mu.RUnlock()

	if len(root) == 0 || len(path) == 0 {
		return
	}

	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return
	}

	current := root
	for seg := range strings.SplitSeq(rel, string(filepath.Separator)) {
		current = filepath.Join(current, seg)
		dt.tree.OpenBranch(current)
	}
	dt.tree.ScrollTo(path)
}

func (dt *DirTree) Widget() *widget.Tree {
	return dt.tree
}

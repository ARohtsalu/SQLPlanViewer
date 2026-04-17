package ui

import (
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type FileTree struct {
	lang          *Lang
	list          *widget.List
	mu            sync.RWMutex
	files         []string
	loadID        int // incremented on each LoadFolder call; goroutine aborts if stale
	currentFolder string
	selectedIndex int
	OnFileSelected func(path string)
	scroll         *container.Scroll
}

func NewFileTree(lang *Lang) *FileTree {
	ft := &FileTree{
		lang:          lang,
		selectedIndex: -1,
	}

	ft.list = widget.NewList(
		func() int {
			ft.mu.RLock()
			n := len(ft.files)
			ft.mu.RUnlock()
			return n
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			ft.mu.RLock()
			if i >= len(ft.files) {
				ft.mu.RUnlock()
				return
			}
			path := ft.files[i]
			ft.mu.RUnlock()
			obj.(*widget.Label).SetText(ft.fileLabel(path))
		},
	)

	ft.list.OnSelected = func(id widget.ListItemID) {
		ft.selectedIndex = id
		ft.mu.RLock()
		if ft.OnFileSelected != nil && id < len(ft.files) {
			path := ft.files[id]
			ft.mu.RUnlock()
			ft.OnFileSelected(path)
			return
		}
		ft.mu.RUnlock()
	}

	ft.scroll = container.NewScroll(ft.list)
	return ft
}

func (ft *FileTree) fileLabel(path string) string {
	name := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(name))

	if ext == ".xdl" {
		return "🔴 " + name
	}

	return "📄 " + name
}

func (ft *FileTree) LoadFolder(dir string) {
	ft.mu.Lock()
	ft.loadID++
	myID := ft.loadID
	ft.currentFolder = dir
	ft.files = nil
	ft.selectedIndex = -1
	ft.mu.Unlock()

	ft.list.Refresh()

	go func() {
		var collected []string
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".sqlplan" || ext == ".xdl" {
				collected = append(collected, path)
			}
			return nil
		})

		ft.mu.Lock()
		if ft.loadID == myID { // discard result if a newer LoadFolder already started
			ft.files = collected
		}
		ft.mu.Unlock()
		ft.list.Refresh()
	}()
}

// LoadFolderAndSelect loads the folder and immediately selects the given file.
// Because loading is async, selection happens after the walk completes.
func (ft *FileTree) LoadFolderAndSelect(dir, filePath string) {
	ft.mu.Lock()
	ft.loadID++
	myID := ft.loadID
	ft.currentFolder = dir
	ft.files = nil
	ft.selectedIndex = -1
	ft.mu.Unlock()

	ft.list.Refresh()

	go func() {
		var collected []string
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".sqlplan" || ext == ".xdl" {
				collected = append(collected, path)
			}
			return nil
		})

		ft.mu.Lock()
		if ft.loadID != myID {
			ft.mu.Unlock()
			return
		}
		ft.files = collected
		ft.mu.Unlock()

		ft.list.Refresh()

		ft.mu.RLock()
		idx := -1
		for i, f := range ft.files {
			if f == filePath {
				idx = i
				break
			}
		}
		ft.mu.RUnlock()

		if idx >= 0 {
			ft.list.Select(idx)
			ft.list.ScrollTo(idx)
		}
	}()
}

func (ft *FileTree) SelectNext() {
	ft.mu.RLock()
	n := len(ft.files)
	ft.mu.RUnlock()
	if n == 0 {
		return
	}
	next := ft.selectedIndex + 1
	if next >= n {
		next = n - 1
	}
	if next != ft.selectedIndex {
		ft.list.Select(next)
	}
}

func (ft *FileTree) SelectPrevious() {
	if ft.selectedIndex <= 0 {
		return
	}
	ft.list.Select(ft.selectedIndex - 1)
}

func (ft *FileTree) SelectedFile() string {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	if ft.selectedIndex < 0 || ft.selectedIndex >= len(ft.files) {
		return ""
	}
	return ft.files[ft.selectedIndex]
}

func (ft *FileTree) SelectedIndex() int {
	return ft.selectedIndex
}

func (ft *FileTree) Count() int {
	ft.mu.RLock()
	n := len(ft.files)
	ft.mu.RUnlock()
	return n
}

func (ft *FileTree) CurrentFolder() string {
	return ft.currentFolder
}

func (ft *FileTree) Widget() fyne.CanvasObject {
	return ft.scroll
}

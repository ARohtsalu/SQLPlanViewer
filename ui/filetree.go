package ui

import (
	"io/fs"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type FileTree struct {
	lang          *Lang
	list          *widget.List
	files         []string
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
		func() int { return len(ft.files) },
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			label.SetText(ft.fileLabel(ft.files[i]))
		},
	)

	ft.list.OnSelected = func(id widget.ListItemID) {
		ft.selectedIndex = id
		if ft.OnFileSelected != nil && id < len(ft.files) {
			ft.OnFileSelected(ft.files[id])
		}
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
	ft.currentFolder = dir
	ft.files = nil
	ft.selectedIndex = -1
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".sqlplan" || ext == ".xdl" {
			ft.files = append(ft.files, path)
		}
		return nil
	})
	ft.list.Refresh()
}

// LoadFolderAndSelect loads the folder and immediately selects the given file.
func (ft *FileTree) LoadFolderAndSelect(dir, filePath string) {
	ft.LoadFolder(dir)
	for i, f := range ft.files {
		if f == filePath {
			ft.list.Select(i)
			ft.list.ScrollTo(i)
			return
		}
	}
}

func (ft *FileTree) SelectNext() {
	if len(ft.files) == 0 {
		return
	}
	next := ft.selectedIndex + 1
	if next >= len(ft.files) {
		next = len(ft.files) - 1
	}
	if next != ft.selectedIndex {
		ft.list.Select(next)
	}
}

func (ft *FileTree) SelectPrevious() {
	if len(ft.files) == 0 {
		return
	}
	prev := ft.selectedIndex - 1
	if prev < 0 {
		prev = 0
	}
	if prev != ft.selectedIndex {
		ft.list.Select(prev)
	}
}

func (ft *FileTree) SelectedFile() string {
	if ft.selectedIndex < 0 || ft.selectedIndex >= len(ft.files) {
		return ""
	}
	return ft.files[ft.selectedIndex]
}

func (ft *FileTree) SelectedIndex() int {
	return ft.selectedIndex
}

func (ft *FileTree) Count() int {
	return len(ft.files)
}

func (ft *FileTree) CurrentFolder() string {
	return ft.currentFolder
}

func (ft *FileTree) Widget() fyne.CanvasObject {
	return ft.scroll
}

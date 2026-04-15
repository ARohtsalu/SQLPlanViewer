package ui

import (
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type FileTree struct {
	lang           *Lang
	list           *widget.List
	files          []string
	selectedIndex  int
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
	ft.files = nil
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
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

func (ft *FileTree) Widget() fyne.CanvasObject {
	return ft.scroll
}

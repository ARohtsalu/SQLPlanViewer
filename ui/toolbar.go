package ui

import (
	"os"
	"os/exec"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

var ssmsPaths = []string{
	`C:\Program Files (x86)\Microsoft SQL Server Management Studio 20\Common7\IDE\Ssms.exe`,
	`C:\Program Files (x86)\Microsoft SQL Server Management Studio 19\Common7\IDE\Ssms.exe`,
	`C:\Program Files\Microsoft SQL Server Management Studio 20\Common7\IDE\Ssms.exe`,
	`C:\Program Files\Microsoft SQL Server Management Studio 19\Common7\IDE\Ssms.exe`,
}

type Toolbar struct {
	container  *fyne.Container
	lang       *Lang
	fileTree   *FileTree
	app        fyne.App
	langBtn    *widget.Button
}

func NewToolbar(lang *Lang, ft *FileTree, a fyne.App) fyne.CanvasObject {
	tb := &Toolbar{lang: lang, fileTree: ft, app: a}

	openBtn := widget.NewButton(lang.T("openFolder"), func() {
		w := a.Driver().AllWindows()[0]
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			ft.LoadFolder(uri.Path())
		}, w)
	})

	tb.langBtn = widget.NewButton("ET | EN", func() {
		lang.Toggle()
		tb.refresh()
	})

	tb.container = container.NewHBox(openBtn, tb.langBtn)
	return tb.container
}

func (tb *Toolbar) refresh() {
	// Rebuild button labels on language toggle
	// Simple approach: update first button text
	if btn, ok := tb.container.Objects[0].(*widget.Button); ok {
		btn.SetText(tb.lang.T("openFolder"))
	}
}

func OpenInSSMS(path string, w fyne.Window, lang *Lang) {
	ssmsPath := ""
	for _, p := range ssmsPaths {
		if _, err := os.Stat(p); err == nil {
			ssmsPath = p
			break
		}
	}

	if ssmsPath == "" {
		entry := widget.NewEntry()
		entry.SetPlaceHolder(`C:\...\Ssms.exe`)
		dialog.ShowCustomConfirm(lang.T("ssmsNotFound"), "OK", "Cancel",
			entry, func(ok bool) {
				if ok && entry.Text != "" {
					_ = exec.Command(entry.Text, path).Start()
				}
			}, w)
		return
	}

	_ = exec.Command(ssmsPath, path).Start()
}

func CopyPathToClipboard(path string, a fyne.App) {
	a.Clipboard().SetContent(path)
}

package ui

import (
	"os"
	"os/exec"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

var defaultSsmsPaths = []string{
	`C:\Program Files (x86)\Microsoft SQL Server Management Studio 20\Common7\IDE\Ssms.exe`,
	`C:\Program Files (x86)\Microsoft SQL Server Management Studio 19\Common7\IDE\Ssms.exe`,
	`C:\Program Files\Microsoft SQL Server Management Studio 20\Common7\IDE\Ssms.exe`,
	`C:\Program Files\Microsoft SQL Server Management Studio 19\Common7\IDE\Ssms.exe`,
}

type Toolbar struct {
	container *fyne.Container
	lang      *Lang
	fileTree  *FileTree
	app       fyne.App
	win       fyne.Window
	settings  *Settings
	langBtn   *widget.Button
}

func NewToolbar(lang *Lang, ft *FileTree, a fyne.App, win fyne.Window, settings *Settings) fyne.CanvasObject {
	tb := &Toolbar{lang: lang, fileTree: ft, app: a, win: win, settings: settings}

	openBtn := widget.NewButton(lang.T("openFolder"), func() {
		fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			filePath := rc.URI().Path()
			rc.Close()
			dir := filepath.Dir(filePath)
			ft.LoadFolderAndSelect(dir, filePath)
		}, win)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".sqlplan", ".xdl"}))
		fd.Show()
	})

	tb.langBtn = widget.NewButton("ET | EN", func() {
		lang.Toggle()
		tb.refresh()
	})

	openSSMS := widget.NewButton(lang.T("openInSSMS"), func() {
		path := ft.SelectedFile()
		if path == "" {
			return
		}
		tb.openInSSMS(path)
	})

	openPS := widget.NewButton(lang.T("openInPS"), func() {
		path := ft.SelectedFile()
		if path == "" {
			return
		}
		tb.openInPS(path)
	})

	copyPath := widget.NewButton(lang.T("copyPath"), func() {
		path := ft.SelectedFile()
		if path != "" {
			a.Clipboard().SetContent(path)
		}
	})

	tb.container = container.NewHBox(openBtn, tb.langBtn, openSSMS, openPS, copyPath)
	return tb.container
}

func (tb *Toolbar) refresh() {
	if btn, ok := tb.container.Objects[0].(*widget.Button); ok {
		btn.SetText(tb.lang.T("openFolder"))
	}
	if btn, ok := tb.container.Objects[2].(*widget.Button); ok {
		btn.SetText(tb.lang.T("openInSSMS"))
	}
	if btn, ok := tb.container.Objects[3].(*widget.Button); ok {
		btn.SetText(tb.lang.T("openInPS"))
	}
	if btn, ok := tb.container.Objects[4].(*widget.Button); ok {
		btn.SetText(tb.lang.T("copyPath"))
	}
}

func (tb *Toolbar) openInSSMS(filePath string) {
	// Use saved path if available
	if tb.settings.SSMSPath != "" {
		if _, err := os.Stat(tb.settings.SSMSPath); err == nil {
			_ = exec.Command(tb.settings.SSMSPath, filePath).Start()
			return
		}
	}

	// Try default install locations
	for _, p := range defaultSsmsPaths {
		if _, err := os.Stat(p); err == nil {
			tb.settings.SSMSPath = p
			SaveSettings(tb.settings)
			_ = exec.Command(p, filePath).Start()
			return
		}
	}

	// Not found — ask user
	entry := widget.NewEntry()
	entry.SetPlaceHolder(`C:\...\Ssms.exe`)
	dialog.ShowCustomConfirm(
		"SSMS path",
		"OK", "Cancel",
		container.NewVBox(
			widget.NewLabel(tb.lang.T("ssmsNotFound")),
			entry,
		),
		func(ok bool) {
			if ok && entry.Text != "" {
				tb.settings.SSMSPath = entry.Text
				SaveSettings(tb.settings)
				_ = exec.Command(entry.Text, filePath).Start()
			}
		},
		tb.win,
	)
}

func (tb *Toolbar) openInPS(filePath string) {
	if tb.settings.PerformanceStudioPath != "" {
		if _, err := os.Stat(tb.settings.PerformanceStudioPath); err == nil {
			_ = exec.Command(tb.settings.PerformanceStudioPath, filePath).Start()
			return
		}
	}

	// Not configured or exe moved — ask user.
	entry := widget.NewEntry()
	entry.SetPlaceHolder(`C:\...\PlanViewer.App.exe`)

	browseBtn := widget.NewButton("Browse...", func() {
		fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			entry.SetText(rc.URI().Path())
			rc.Close()
		}, tb.win)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".exe"}))
		fd.Show()
	})

	dialog.ShowCustomConfirm(
		"Performance Studio",
		"OK", "Cancel",
		container.NewVBox(
			widget.NewLabel(tb.lang.T("psNotFound")),
			container.NewBorder(nil, nil, nil, browseBtn, entry),
		),
		func(ok bool) {
			if ok && entry.Text != "" {
				tb.settings.PerformanceStudioPath = entry.Text
				SaveSettings(tb.settings)
				_ = exec.Command(entry.Text, filePath).Start()
			}
		},
		tb.win,
	)
}

// OpenInSSMS is kept for external callers (backward compat).
func OpenInSSMS(path string, w fyne.Window, lang *Lang) {
	for _, p := range defaultSsmsPaths {
		if _, err := os.Stat(p); err == nil {
			_ = exec.Command(p, path).Start()
			return
		}
	}
	entry := widget.NewEntry()
	entry.SetPlaceHolder(`C:\...\Ssms.exe`)
	dialog.ShowCustomConfirm(lang.T("ssmsNotFound"), "OK", "Cancel",
		entry, func(ok bool) {
			if ok && entry.Text != "" {
				_ = exec.Command(entry.Text, path).Start()
			}
		}, w)
}

func CopyPathToClipboard(path string, a fyne.App) {
	a.Clipboard().SetContent(path)
}

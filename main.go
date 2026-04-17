package main

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"

	"sqlplanviewer/ui"
)

func main() {
	a := app.NewWithID("io.github.arohtsalu.sqlplanviewer")
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("SQL Plan Viewer")
	w.Resize(fyne.NewSize(1400, 800))
	w.SetFixedSize(false)

	settings := ui.LoadSettings()

	lang := ui.NewLang(settings.Language)

	fileTree := ui.NewFileTree(lang)
	planView := ui.NewPlanView(lang, w)
	statusBar := ui.NewStatusBar()

	fileTree.OnFileSelected = func(path string) {
		planView.Load(path)
		w.SetTitle("SQL Plan Viewer — " + filepath.Base(path))
		statusBar.Update(filepath.Base(path), fileTree.SelectedIndex()+1, fileTree.Count())
	}

	// Keyboard navigation
	w.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		switch ev.Name {
		case fyne.KeyUp:
			fileTree.SelectPrevious()
		case fyne.KeyDown:
			fileTree.SelectNext()
		}
	})

	split := container.NewHSplit(fileTree.Widget(), planView.Widget())
	split.SetOffset(0.25)

	toolbar := ui.NewToolbar(lang, fileTree, a, w, settings)

	content := container.NewBorder(toolbar, statusBar.Widget(), nil, nil, split)
	w.SetContent(content)

	// Restore last folder
	if settings.LastFolder != "" {
		fileTree.LoadFolder(settings.LastFolder)
	}

	// Save settings on close
	w.SetOnClosed(func() {
		settings.LastFolder = fileTree.CurrentFolder()
		settings.Language = lang.Code()
		ui.SaveSettings(settings)
	})

	w.ShowAndRun()
}

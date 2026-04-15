package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"

	"sqlplanviewer/ui"
)

func main() {
	a := app.New()
	w := a.NewWindow("SQL Plan Viewer")
	w.Resize(fyne.NewSize(1100, 700))

	lang := ui.NewLang("EN")

	fileTree := ui.NewFileTree(lang)
	planView := ui.NewPlanView(lang)

	fileTree.OnFileSelected = func(path string) {
		planView.Load(path)
	}

	split := container.NewHSplit(fileTree.Widget(), planView.Widget())
	split.SetOffset(0.28)

	toolbar := ui.NewToolbar(lang, fileTree, a)

	content := container.NewBorder(toolbar, nil, nil, nil, split)
	w.SetContent(content)
	w.ShowAndRun()
}

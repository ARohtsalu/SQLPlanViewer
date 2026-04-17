package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type StatusBar struct {
	label *widget.Label
}

func NewStatusBar() *StatusBar {
	sb := &StatusBar{
		label: widget.NewLabel(""),
	}
	return sb
}

func (sb *StatusBar) Update(filename string, current, total int) {
	if filename == "" {
		sb.label.SetText("")
		return
	}
	sb.label.SetText(fmt.Sprintf(
		"%s   |   File %d / %d   |   ↑↓ arrow keys to navigate",
		filename, current, total))
}

func (sb *StatusBar) Widget() fyne.CanvasObject {
	return sb.label
}

package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"sqlplanviewer/parser"
)

type PlanView struct {
	lang      *Lang
	content   *fyne.Container
	wrapper   *container.Scroll
}

func NewPlanView(lang *Lang) *PlanView {
	pv := &PlanView{lang: lang}
	placeholder := widget.NewLabel(lang.T("noFile"))
	pv.content = container.NewVBox(placeholder)
	pv.wrapper = container.NewScroll(pv.content)
	return pv
}

func (pv *PlanView) Load(path string) {
	ext := strings.ToLower(filepath.Ext(path))
	pv.content.Objects = nil

	switch ext {
	case ".sqlplan":
		pv.loadSqlPlan(path)
	case ".xdl":
		pv.loadDeadlock(path)
	default:
		pv.content.Objects = []fyne.CanvasObject{
			widget.NewLabel("Unsupported file type: " + ext),
		}
	}
	pv.content.Refresh()
}

func (pv *PlanView) loadSqlPlan(path string) {
	qp, err := parser.ParseSqlPlan(path)
	if err != nil {
		pv.content.Objects = []fyne.CanvasObject{
			widget.NewLabel("Error: " + err.Error()),
		}
		return
	}

	stmt := qp.StatementText
	if len(stmt) > 200 {
		stmt = stmt[:200] + "..."
	}

	stmtLabel := widget.NewLabel("Statement: " + stmt)
	stmtLabel.Wrapping = fyne.TextWrapWord

	costLabel := widget.NewLabel(fmt.Sprintf("Est. Cost: %.4f   |   Est. Rows: %.0f",
		qp.TotalCost, qp.EstimatedRows))
	costLabel.TextStyle = fyne.TextStyle{Bold: true}

	graph := NewPlanGraph(qp, pv.lang)

	alertItems := []fyne.CanvasObject{}
	if len(qp.Warnings) > 0 {
		alertItems = append(alertItems,
			widget.NewLabel(fmt.Sprintf("⚠️  %s: %d", pv.lang.T("warnings"), len(qp.Warnings))))
	}
	if len(qp.MissingIndexes) > 0 {
		alertItems = append(alertItems,
			widget.NewLabel(fmt.Sprintf("🔍 %s: %d", pv.lang.T("missingIndexes"), len(qp.MissingIndexes))))
	}
	if qp.RootOp != nil {
		scans := countOp(qp.RootOp, "Table Scan")
		if scans > 0 {
			alertItems = append(alertItems,
				widget.NewLabel(fmt.Sprintf("🔴 %s: %d", pv.lang.T("tableScan"), scans)))
		}
	}

	objects := []fyne.CanvasObject{stmtLabel, costLabel, graph.Widget()}
	if len(alertItems) > 0 {
		objects = append(objects, alertItems...)
	}

	pv.content.Objects = objects
}

func (pv *PlanView) loadDeadlock(path string) {
	dg, err := parser.ParseDeadlock(path)
	if err != nil {
		pv.content.Objects = []fyne.CanvasObject{
			widget.NewLabel("Error: " + err.Error()),
		}
		return
	}

	title := widget.NewLabel("🔴 " + pv.lang.T("deadlockVictim") + ": " + dg.Victim)
	title.TextStyle = fyne.TextStyle{Bold: true}

	procHeader := widget.NewLabel(pv.lang.T("processes") + ":")
	procHeader.TextStyle = fyne.TextStyle{Bold: true}

	objects := []fyne.CanvasObject{title, procHeader}

	for _, p := range dg.Processes {
		prefix := "  "
		if p.IsVictim {
			prefix = "💀 "
		}
		label := fmt.Sprintf("%sSPID %d  |  %s  |  Wait: %s", prefix, p.SPID, p.Login, p.WaitResource)
		lbl := widget.NewLabel(label)
		objects = append(objects, lbl)
		if p.QueryText != "" {
			q := strings.TrimSpace(p.QueryText)
			if len(q) > 150 {
				q = q[:150] + "..."
			}
			objects = append(objects, widget.NewLabel("    "+q))
		}
	}

	resHeader := widget.NewLabel(pv.lang.T("resources") + ":")
	resHeader.TextStyle = fyne.TextStyle{Bold: true}
	objects = append(objects, resHeader)

	for _, r := range dg.Resources {
		objects = append(objects, widget.NewLabel(
			fmt.Sprintf("  [%s] %s  mode: %s", r.Type, r.ObjectName, r.Mode)))
	}

	pv.content.Objects = objects
}

func countOp(op *parser.RelOp, name string) int {
	if op == nil {
		return 0
	}
	n := 0
	if op.PhysicalOp == name {
		n = 1
	}
	for _, c := range op.Children {
		n += countOp(c, name)
	}
	return n
}

func (pv *PlanView) Widget() fyne.CanvasObject {
	return pv.wrapper
}

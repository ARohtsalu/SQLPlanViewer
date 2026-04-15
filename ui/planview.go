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
	lang    *Lang
	wrapper fyne.CanvasObject
	replace func(fyne.CanvasObject)
}

func NewPlanView(lang *Lang) *PlanView {
	pv := &PlanView{lang: lang}
	placeholder := widget.NewLabel(lang.T("noFile"))
	// Use a border container so the inner content fills the space
	inner := container.NewBorder(nil, nil, nil, nil, placeholder)
	pv.wrapper = inner
	return pv
}

func (pv *PlanView) Load(path string) {
	ext := strings.ToLower(filepath.Ext(path))
	var content fyne.CanvasObject

	switch ext {
	case ".sqlplan":
		content = pv.buildSqlPlanView(path)
	case ".xdl":
		content = pv.buildDeadlockView(path)
	default:
		content = widget.NewLabel("Unsupported file type: " + ext)
	}

	// Replace inner content: build a new border container
	inner := pv.wrapper.(*fyne.Container)
	inner.Objects = []fyne.CanvasObject{content}
	inner.Refresh()
}

func (pv *PlanView) buildSqlPlanView(path string) fyne.CanvasObject {
	qp, err := parser.ParseSqlPlan(path)
	if err != nil {
		return widget.NewLabel("Parse error: " + err.Error())
	}
	if qp.RootOp == nil {
		return widget.NewLabel("No query plan found in file")
	}

	// Info panel (top, fixed height)
	stmt := qp.StatementText
	if len(stmt) > 200 {
		stmt = stmt[:200] + "..."
	}
	stmtLbl := widget.NewLabel("SQL: " + stmt)
	stmtLbl.Wrapping = fyne.TextWrapWord

	costLbl := widget.NewLabel(fmt.Sprintf("Cost: %.4f   Est. Rows: %.0f   Nodes: %d",
		qp.TotalCost, qp.EstimatedRows, parser.CountNodes(qp.RootOp)))
	costLbl.TextStyle = fyne.TextStyle{Bold: true}

	alerts := []fyne.CanvasObject{}
	if len(qp.Warnings) > 0 {
		alerts = append(alerts, widget.NewLabel(fmt.Sprintf("⚠️  %s: %d", pv.lang.T("warnings"), len(qp.Warnings))))
	}
	if len(qp.MissingIndexes) > 0 {
		alerts = append(alerts, widget.NewLabel(fmt.Sprintf("🔍 %s: %d", pv.lang.T("missingIndexes"), len(qp.MissingIndexes))))
	}
	scans := countOpType(qp.RootOp, "Table Scan")
	if scans > 0 {
		alerts = append(alerts, widget.NewLabel(fmt.Sprintf("🔴 %s: %d", pv.lang.T("tableScan"), scans)))
	}

	infoItems := []fyne.CanvasObject{stmtLbl, costLbl}
	infoItems = append(infoItems, alerts...)
	infoPanel := container.NewVBox(infoItems...)

	// Graph panel fills the rest
	graph := NewPlanGraph(qp, pv.lang)
	graphWidget := graph.Widget()

	return container.NewBorder(infoPanel, nil, nil, nil, graphWidget)
}

func (pv *PlanView) buildDeadlockView(path string) fyne.CanvasObject {
	dg, err := parser.ParseDeadlock(path)
	if err != nil {
		return widget.NewLabel("Parse error: " + err.Error())
	}

	// Info panel
	victimLabel := widget.NewLabel(fmt.Sprintf("🔴 %s  |  %d processes  |  %d resources",
		pv.lang.T("deadlockVictim"), len(dg.Processes), len(dg.Resources)))
	victimLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Find victim process for extra info
	for _, p := range dg.Processes {
		if p.IsVictim {
			victimLabel.SetText(fmt.Sprintf("💀 Victim: SPID %d (%s)  |  %d processes  |  %d resources",
				p.SPID, p.Login, len(dg.Processes), len(dg.Resources)))
			break
		}
	}

	infoPanel := container.NewVBox(victimLabel)
	graphWidget := NewDeadlockGraph(dg, pv.lang)

	return container.NewBorder(infoPanel, nil, nil, nil, graphWidget)
}

func countOpType(op *parser.RelOp, name string) int {
	if op == nil {
		return 0
	}
	n := 0
	if op.PhysicalOp == name {
		n = 1
	}
	for _, c := range op.Children {
		n += countOpType(c, name)
	}
	return n
}

func (pv *PlanView) Widget() fyne.CanvasObject {
	return pv.wrapper
}

// Helper: trim query text
func trimQuery(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

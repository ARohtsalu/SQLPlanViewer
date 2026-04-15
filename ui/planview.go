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
	wrapper *fyne.Container
}

func NewPlanView(lang *Lang) *PlanView {
	pv := &PlanView{lang: lang}
	placeholder := widget.NewLabel(lang.T("noFile"))
	pv.wrapper = container.NewBorder(nil, nil, nil, nil, placeholder)
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

	pv.wrapper.Objects = []fyne.CanvasObject{content}
	pv.wrapper.Refresh()
}

func (pv *PlanView) buildSqlPlanView(path string) fyne.CanvasObject {
	plans, err := parser.ParseSqlPlan(path)
	if err != nil {
		return widget.NewLabel("Parse error: " + err.Error())
	}
	if len(plans) == 0 {
		return widget.NewLabel("No statements found in file")
	}

	batchTotal := parser.BatchTotal(plans)

	// Build one tab per statement
	tabs := container.NewAppTabs()
	for _, plan := range plans {
		plan := plan // capture loop var

		pct := 0.0
		if batchTotal > 0 {
			pct = plan.TotalCost / batchTotal * 100
		}
		label := fmt.Sprintf("Query %d: %.0f%%", plan.StatementIndex, pct)

		var tabContent fyne.CanvasObject
		if plan.RootOp == nil {
			// DDL / SET statement — no execution plan
			stmt := strings.TrimSpace(plan.StatementText)
			if len(stmt) > 120 {
				stmt = stmt[:120] + "..."
			}
			tabContent = container.NewVBox(
				widget.NewLabel("(no execution plan — DDL or SET statement)"),
				widget.NewLabel(stmt),
			)
		} else {
			tabContent = pv.buildPlanTab(plan, batchTotal)
		}

		tabs.Append(container.NewTabItem(label, tabContent))
	}

	// Auto-select most expensive statement
	bestIdx := parser.MostExpensiveIndex(plans)
	tabs.SelectIndex(bestIdx)

	return tabs
}

func (pv *PlanView) buildPlanTab(plan *parser.QueryPlan, batchTotal float64) fyne.CanvasObject {
	stmt := strings.TrimSpace(plan.StatementText)
	if len(stmt) > 200 {
		stmt = stmt[:200] + "..."
	}
	stmtLbl := widget.NewLabel("SQL: " + stmt)
	stmtLbl.Wrapping = fyne.TextWrapWord

	pct := 0.0
	if batchTotal > 0 {
		pct = plan.TotalCost / batchTotal * 100
	}
	costLbl := widget.NewLabel(fmt.Sprintf(
		"Cost: %.4f (%.0f%% of batch)   Est. Rows: %.0f   Nodes: %d",
		plan.TotalCost, pct, plan.EstimatedRows, parser.CountNodes(plan.RootOp)))
	costLbl.TextStyle = fyne.TextStyle{Bold: true}

	alerts := []fyne.CanvasObject{}
	if len(plan.Warnings) > 0 {
		alerts = append(alerts, widget.NewLabel(
			fmt.Sprintf("⚠️  %s: %d", pv.lang.T("warnings"), len(plan.Warnings))))
	}
	if len(plan.MissingIndexes) > 0 {
		alerts = append(alerts, widget.NewLabel(
			fmt.Sprintf("🔍 %s: %d", pv.lang.T("missingIndexes"), len(plan.MissingIndexes))))
	}
	scans := countOpType(plan.RootOp, "Table Scan")
	if scans > 0 {
		alerts = append(alerts, widget.NewLabel(
			fmt.Sprintf("🔴 %s: %d", pv.lang.T("tableScan"), scans)))
	}

	infoItems := []fyne.CanvasObject{stmtLbl, costLbl}
	infoItems = append(infoItems, alerts...)
	infoPanel := container.NewVBox(infoItems...)

	graph := NewPlanGraph(plan, pv.lang)
	return container.NewBorder(infoPanel, nil, nil, nil, graph.Widget())
}

func (pv *PlanView) buildDeadlockView(path string) fyne.CanvasObject {
	dg, err := parser.ParseDeadlock(path)
	if err != nil {
		return widget.NewLabel("Parse error: " + err.Error())
	}

	// Top info panel (~180px): structured deadlock summary
	infoPanel := BuildDeadlockInfoPanel(dg)
	infoScroll := container.NewVScroll(infoPanel)
	infoScroll.SetMinSize(fyne.NewSize(0, 180))

	// Bottom: visual graph
	graphWidget := NewDeadlockGraph(dg, pv.lang)

	return container.NewBorder(infoScroll, nil, nil, nil, graphWidget)
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

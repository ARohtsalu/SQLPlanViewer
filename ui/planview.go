package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"image/color"

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

	tabs := container.NewAppTabs()

	// First tab: "All Queries" overview
	allView := pv.buildAllStatementsView(plans, batchTotal)
	tabs.Append(container.NewTabItem("All Queries", allView))

	// One tab per statement
	for _, plan := range plans {
		plan := plan
		pct := 0.0
		if batchTotal > 0 {
			pct = plan.TotalCost / batchTotal * 100
		}
		label := fmt.Sprintf("Q%d: %.0f%%", plan.StatementIndex, pct)

		var tabContent fyne.CanvasObject
		if plan.RootOp == nil {
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

	// Select the most expensive statement tab (offset by 1 for "All Queries")
	bestIdx := parser.MostExpensiveIndex(plans)
	tabs.SelectIndex(bestIdx + 1)

	return tabs
}

func (pv *PlanView) buildAllStatementsView(plans []*parser.QueryPlan, batchTotal float64) fyne.CanvasObject {
	rows := []fyne.CanvasObject{}

	for i, plan := range plans {
		pct := 0.0
		if batchTotal > 0 {
			pct = plan.TotalCost / batchTotal * 100
		}

		header := widget.NewLabel(fmt.Sprintf(
			"Query %d: Query cost (relative to the batch): %.0f%%",
			plan.StatementIndex, pct))
		header.TextStyle = fyne.TextStyle{Bold: true}

		// Full SQL text — scrollable, read-only
		sqlEntry := widget.NewMultiLineEntry()
		sqlEntry.SetText(strings.TrimSpace(plan.StatementText))
		sqlEntry.Wrapping = fyne.TextWrapWord
		sqlEntry.Disable()
		sqlScroll := container.NewScroll(sqlEntry)
		sqlScroll.SetMinSize(fyne.NewSize(0, 70))

		rows = append(rows, header, sqlScroll)

		// Mini plan graph
		if plan.RootOp != nil {
			g := NewPlanGraph(plan, pv.lang)
			mini := g.MiniWidget()
			miniScroll := container.NewVScroll(mini)
			miniScroll.SetMinSize(fyne.NewSize(0, 180))
			rows = append(rows, miniScroll)
		} else {
			rows = append(rows, widget.NewLabel("(no execution plan)"))
		}

		// Separator line between statements
		if i < len(plans)-1 {
			sep := canvas.NewLine(color.RGBA{R: 180, G: 180, B: 180, A: 255})
			sep.StrokeWidth = 1
			rows = append(rows, sep)
		}
	}

	return container.NewScroll(container.NewVBox(rows...))
}

func (pv *PlanView) buildPlanTab(plan *parser.QueryPlan, batchTotal float64) fyne.CanvasObject {
	pct := 0.0
	if batchTotal > 0 {
		pct = plan.TotalCost / batchTotal * 100
	}

	// Full SQL — scrollable, read-only
	sqlEntry := widget.NewMultiLineEntry()
	sqlEntry.SetText(strings.TrimSpace(plan.StatementText))
	sqlEntry.Wrapping = fyne.TextWrapWord
	sqlEntry.Disable()
	sqlScroll := container.NewScroll(sqlEntry)
	sqlScroll.SetMinSize(fyne.NewSize(0, 80))

	costLbl := widget.NewLabel(fmt.Sprintf(
		"Cost: %.4f (%.0f%% of batch)   Est. Rows: %s   Nodes: %d",
		plan.TotalCost, pct,
		formatRows(plan.EstimatedRows),
		parser.CountNodes(plan.RootOp)))
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

	infoItems := []fyne.CanvasObject{sqlScroll, costLbl}
	infoItems = append(infoItems, alerts...)
	infoPanel := container.NewVBox(infoItems...)

	graph := NewPlanGraph(plan, pv.lang)
	graphWidget := graph.Widget()

	// VSplit: info top (resizable), graph bottom
	vsplit := container.NewVSplit(infoPanel, graphWidget)
	vsplit.SetOffset(0.2)
	return vsplit
}

func (pv *PlanView) buildDeadlockView(path string) fyne.CanvasObject {
	dg, err := parser.ParseDeadlock(path)
	if err != nil {
		return widget.NewLabel("Parse error: " + err.Error())
	}

	infoPanel := BuildDeadlockInfoPanel(dg)
	graphWidget := NewDeadlockGraph(dg, pv.lang)

	vsplit := container.NewVSplit(infoPanel, graphWidget)
	vsplit.SetOffset(0.3)
	return vsplit
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

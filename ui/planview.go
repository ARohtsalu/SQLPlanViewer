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
	win     fyne.Window
	wrapper *fyne.Container
}

func NewPlanView(lang *Lang, win fyne.Window) *PlanView {
	pv := &PlanView{lang: lang, win: win}
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

	// Single statement: skip the tab bar entirely.
	if len(plans) == 1 {
		return pv.buildPlanContent(plans[0], batchTotal)
	}

	// ── Multi-statement: custom wrapping tab bar ──────────────────────────────
	// GridWrap: each tab has a fixed minimum size; Fyne wraps automatically
	// so more tabs fit per row as the window widens.

	// Base importance per plan (restored when tab is deselected).
	baseImp := func(p *parser.QueryPlan) widget.ButtonImportance {
		if batchTotal == 0 || p.TotalCost == 0 {
			return widget.LowImportance
		}
		pct := p.TotalCost / batchTotal * 100
		switch {
		case pct >= 25:
			return widget.DangerImportance
		case pct >= 10:
			return widget.WarningImportance
		default:
			return widget.MediumImportance
		}
	}

	built := make([]fyne.CanvasObject, len(plans))
	contentArea := container.NewBorder(nil, nil, nil, nil, widget.NewLabel(""))
	currentIdx := -1

	var buttons []*widget.Button

	selectTab := func(idx int) {
		if idx == currentIdx {
			return
		}
		if built[idx] == nil {
			built[idx] = pv.buildPlanContent(plans[idx], batchTotal)
		}
		contentArea.Objects = []fyne.CanvasObject{built[idx]}
		contentArea.Refresh()

		for i, btn := range buttons {
			if i == idx {
				btn.Importance = widget.HighImportance
			} else {
				btn.Importance = baseImp(plans[i])
			}
			btn.Refresh()
		}
		currentIdx = idx
	}

	tabItems := make([]fyne.CanvasObject, len(plans))
	buttons = make([]*widget.Button, len(plans))
	for i, plan := range plans {
		idx := i
		pct := 0.0
		if batchTotal > 0 {
			pct = plan.TotalCost / batchTotal * 100
		}
		label := fmt.Sprintf("Q%d: %.0f%%", plan.StatementIndex, pct)
		btn := widget.NewButton(label, func() { selectTab(idx) })
		btn.Importance = baseImp(plan)
		buttons[i] = btn
		tabItems[i] = btn
	}

	// GridWrap: tabs wrap to next row when the row is full.
	// Each tab is min 90×30 px; the actual width grows to fill the grid cell.
	tabBar := container.NewGridWrap(fyne.NewSize(90, 30), tabItems...)

	// Auto-select the most expensive statement.
	best := parser.MostExpensiveIndex(plans)
	selectTab(best)

	return container.NewBorder(tabBar, nil, nil, nil, contentArea)
}

// buildPlanContent builds the content for one statement (plan graph + info panel).
// Extracted so both single-statement and multi-statement paths share the same code.
func (pv *PlanView) buildPlanContent(plan *parser.QueryPlan, batchTotal float64) fyne.CanvasObject {
	if plan.RootOp == nil {
		stmt := strings.TrimSpace(plan.StatementText)
		if len(stmt) > 200 {
			stmt = stmt[:200] + "..."
		}
		return container.NewVBox(
			widget.NewLabel("(no execution plan — DDL or SET statement)"),
			widget.NewLabel(stmt),
		)
	}
	return pv.buildPlanTab(plan, batchTotal)
}

func (pv *PlanView) buildPlanTab(plan *parser.QueryPlan, batchTotal float64) fyne.CanvasObject {
	pct := 0.0
	if batchTotal > 0 {
		pct = plan.TotalCost / batchTotal * 100
	}

	// Full SQL — scrollable label (Label uses theme foreground, readable in dark theme)
	sqlLabel := widget.NewLabel(strings.TrimSpace(plan.StatementText))
	sqlLabel.Wrapping = fyne.TextWrapWord
	sqlScroll := container.NewScroll(sqlLabel)
	sqlScroll.SetMinSize(fyne.NewSize(0, 80))

	costLbl := widget.NewLabel(fmt.Sprintf(
		"Cost: %.4f (%.0f%% of batch)   Est. Rows: %s   Nodes: %d",
		plan.TotalCost, pct,
		formatRows(plan.EstimatedRows),
		parser.CountNodes(plan.RootOp)))
	costLbl.TextStyle = fyne.TextStyle{Bold: true}

	alerts := []fyne.CanvasObject{}
	for _, mi := range plan.MissingIndexes {
		db := strings.Trim(mi.Database, "[]")
		tbl := strings.Trim(mi.Table, "[]")
		cols := mi.Columns
		if cols == "" {
			cols = "—"
		}
		lbl := widget.NewLabel(fmt.Sprintf("🔍 %s.%s — %s  (Impact: %.0f%%)", db, tbl, cols, mi.Impact))
		lbl.Wrapping = fyne.TextWrapWord
		alerts = append(alerts, lbl)
	}
	for _, w := range plan.Warnings {
		alerts = append(alerts, widget.NewLabel("⚠ "+w.Text))
	}
	scans := parser.CountOp(plan.RootOp, "Table Scan")
	if scans > 0 {
		alerts = append(alerts, widget.NewLabel(
			fmt.Sprintf("🔴 %s: %d", pv.lang.T("tableScan"), scans)))
	}

	infoItems := []fyne.CanvasObject{sqlScroll, costLbl}
	infoItems = append(infoItems, alerts...)
	infoPanel := container.NewVBox(infoItems...)

	graph := NewPlanGraph(plan, pv.lang, pv.win)
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


func (pv *PlanView) Widget() fyne.CanvasObject {
	return pv.wrapper
}

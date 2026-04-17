package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"sqlplanviewer/parser"
)

// BuildTooltipContent builds an SSMS-style tooltip for a plan node.
func BuildTooltipContent(node *parser.RelOp) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(
		node.PhysicalOp,
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	rows := []fyne.CanvasObject{title}

	desc := operatorDescription(node.PhysicalOp)
	if desc != "" {
		d := widget.NewLabel(desc)
		d.Wrapping = fyne.TextWrapWord
		rows = append(rows, d)
	}

	rows = append(rows, widget.NewSeparator())

	addRow := func(label, value string) {
		if value == "" || value == "0" || value == "0.0000000" {
			return
		}
		lbl := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		val := widget.NewLabel(value)
		rows = append(rows, container.NewGridWithColumns(2, lbl, val))
	}

	addRow("Physical Operation", node.PhysicalOp)
	addRow("Logical Operation", node.LogicalOp)
	if node.ExecutionMode != "" {
		addRow("Execution Mode", node.ExecutionMode)
	}
	if node.ObjStorage != "" {
		addRow("Storage", node.ObjStorage)
	}
	addRow("Estimated Operator Cost",
		fmt.Sprintf("%.7f (%.0f%%)", node.EstimatedCost, node.CostPercent))
	if node.EstimateIO > 0 {
		addRow("Estimated I/O Cost", fmt.Sprintf("%.7f", node.EstimateIO))
	}
	if node.EstimateCPU > 0 {
		addRow("Estimated CPU Cost", fmt.Sprintf("%.7f", node.EstimateCPU))
	}
	addRow("Estimated Subtree Cost", fmt.Sprintf("%.7f", node.EstimatedCost))
	addRow("Est. Number of Rows", fmt.Sprintf("%.4f", node.EstimatedRows))
	if node.EstRowsRead > 0 {
		addRow("Est. Rows to be Read", fmt.Sprintf("%.4f", node.EstRowsRead))
	}
	if node.AvgRowSize > 0 {
		addRow("Estimated Row Size", fmt.Sprintf("%.0f B", node.AvgRowSize))
	}
	if node.TableCardinality > 0 {
		addRow("Table Cardinality", fmt.Sprintf("%.0f", node.TableCardinality))
	}
	if node.EstRebinds > 0 {
		addRow("Est. Rebinds", fmt.Sprintf("%.4f", node.EstRebinds))
	}
	if node.EstRewinds > 0 {
		addRow("Est. Rewinds", fmt.Sprintf("%.4f", node.EstRewinds))
	}
	addRow("Node ID", fmt.Sprintf("%d", node.NodeID))
	if node.Parallel {
		addRow("Parallel", "True")
	}
	if node.OutputColumns > 0 {
		addRow("Output Columns", fmt.Sprintf("%d", node.OutputColumns))
	}

	// Object info section
	if node.ObjTable != "" {
		rows = append(rows, widget.NewSeparator())
		rows = append(rows, widget.NewLabelWithStyle(
			"Object", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		objText := fmt.Sprintf("%s.%s.%s", node.ObjDatabase, node.ObjSchema, node.ObjTable)
		if node.ObjIndex != "" {
			objText += "." + node.ObjIndex
		}
		if node.ObjIndexKind != "" {
			objText += " (" + node.ObjIndexKind + ")"
		}
		objLabel := widget.NewLabel(objText)
		objLabel.Wrapping = fyne.TextWrapWord
		rows = append(rows, objLabel)
	}

	// Warnings section
	if node.HasWarnings {
		rows = append(rows, widget.NewSeparator())
		for _, w := range node.WarningTexts {
			wlbl := widget.NewLabelWithStyle("⚠ "+w,
				fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
			wlbl.Wrapping = fyne.TextWrapWord
			rows = append(rows, wlbl)
		}
	}

	box := container.NewVBox(rows...)

	// Dark background — matches the app dark theme; labels use theme foreground (light) which is readable on dark.
	bg := canvas.NewRectangle(color.RGBA{R: 0x1E, G: 0x22, B: 0x2B, A: 250})
	return container.NewStack(bg, container.NewPadded(box))
}

func operatorDescription(physicalOp string) string {
	switch physicalOp {
	case "Index Seek", "Clustered Index Seek":
		return "Scan a particular range of rows from an index."
	case "Index Scan", "Clustered Index Scan":
		return "Scanning an entire index. May indicate missing WHERE clause or poor index coverage."
	case "Table Scan":
		return "Scanning the entire table. No usable index found — consider adding an index."
	case "Key Lookup":
		return "A lookup into a clustered index to retrieve columns not in the nonclustered index."
	case "Hash Match":
		return "Uses a hash table to match rows. Common for joins and aggregations with large data sets."
	case "Nested Loops":
		return "For each row in the outer input, scans the inner input. Efficient for small row counts."
	case "Sort":
		return "Sorts input rows. Can spill to TempDB if memory grant is insufficient."
	case "Table Spool", "Row Count Spool":
		return "Stores intermediate results in a worktable in TempDB."
	case "Compute Scalar":
		return "Computes a scalar expression for each row."
	case "Stream Aggregate":
		return "Computes aggregation on pre-sorted input."
	case "Filter":
		return "Applies a residual predicate to filter rows."
	case "Top":
		return "Returns only the first N rows from input."
	case "Table Insert", "Clustered Index Insert":
		return "Inserts rows into a table or clustered index."
	default:
		return ""
	}
}

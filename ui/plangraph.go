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

const (
	nodeW    = 160
	nodeH    = 60
	nodeGapX = 40
	nodeGapY = 30
)

type PlanGraph struct {
	plan   *parser.QueryPlan
	lang   *Lang
}

func NewPlanGraph(plan *parser.QueryPlan, lang *Lang) *PlanGraph {
	return &PlanGraph{plan: plan, lang: lang}
}

func (pg *PlanGraph) Widget() fyne.CanvasObject {
	if pg.plan.RootOp == nil {
		return widget.NewLabel("(no plan graph)")
	}

	// Assign positions using a simple recursive layout
	positions := map[int]fyne.Position{}
	sizes := map[int]float32{}
	assignX(pg.plan.RootOp, 10, positions, sizes)
	assignY(pg.plan.RootOp, 10, positions)

	totalW, totalH := graphBounds(positions)

	objects := []fyne.CanvasObject{}
	drawEdges(pg.plan.RootOp, positions, &objects)
	drawNodes(pg.plan.RootOp, pg.plan.TotalCost, positions, &objects)

	c := container.NewWithoutLayout(objects...)
	c.Resize(fyne.NewSize(totalW+nodeW+20, totalH+nodeH+20))
	return container.NewScroll(c)
}

// assignX returns the total width used by this subtree
func assignX(op *parser.RelOp, startX float32, positions map[int]fyne.Position, sizes map[int]float32) float32 {
	if op == nil {
		return 0
	}
	if len(op.Children) == 0 {
		sizes[op.NodeID] = nodeW
		positions[op.NodeID] = fyne.NewPos(startX, 0) // Y set later
		return nodeW
	}

	x := startX
	totalW := float32(0)
	for _, child := range op.Children {
		w := assignX(child, x, positions, sizes)
		x += w + nodeGapX
		totalW += w + nodeGapX
	}
	totalW -= nodeGapX

	// Center parent over children
	firstChildX := positions[op.Children[0].NodeID].X
	lastChildX := positions[op.Children[len(op.Children)-1].NodeID].X
	centerX := (firstChildX + lastChildX) / 2
	positions[op.NodeID] = fyne.NewPos(centerX, 0)
	sizes[op.NodeID] = totalW
	return totalW
}

func assignY(op *parser.RelOp, startY float32, positions map[int]fyne.Position) {
	if op == nil {
		return
	}
	pos := positions[op.NodeID]
	positions[op.NodeID] = fyne.NewPos(pos.X, startY)
	for _, child := range op.Children {
		assignY(child, startY+nodeH+nodeGapY, positions)
	}
}

func graphBounds(positions map[int]fyne.Position) (float32, float32) {
	var maxX, maxY float32
	for _, p := range positions {
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	return maxX, maxY
}

func drawEdges(op *parser.RelOp, positions map[int]fyne.Position, objects *[]fyne.CanvasObject) {
	if op == nil {
		return
	}
	parentPos := positions[op.NodeID]
	px := parentPos.X + nodeW/2
	py := parentPos.Y + nodeH

	for _, child := range op.Children {
		childPos := positions[child.NodeID]
		cx := childPos.X + nodeW/2
		cy := childPos.Y

		line := canvas.NewLine(color.Gray{Y: 120})
		line.StrokeWidth = 1.5
		line.Position1 = fyne.NewPos(px, py)
		line.Position2 = fyne.NewPos(cx, cy)
		*objects = append(*objects, line)

		drawEdges(child, positions, objects)
	}
}

func drawNodes(op *parser.RelOp, totalCost float64, positions map[int]fyne.Position, objects *[]fyne.CanvasObject) {
	if op == nil {
		return
	}
	pos := positions[op.NodeID]
	col := nodeColor(op)

	bg := canvas.NewRectangle(col)
	bg.CornerRadius = 6
	bg.Resize(fyne.NewSize(nodeW, nodeH))
	bg.Move(pos)

	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = color.Gray{Y: 80}
	border.StrokeWidth = 1
	border.CornerRadius = 6
	border.Resize(fyne.NewSize(nodeW, nodeH))
	border.Move(pos)

	opLabel := canvas.NewText(op.PhysicalOp, color.Black)
	opLabel.TextSize = 11
	opLabel.TextStyle = fyne.TextStyle{Bold: true}
	opLabel.Move(fyne.NewPos(pos.X+6, pos.Y+6))

	infoText := fmt.Sprintf("%.1f%%  rows:%.0f", op.CostPercent, op.EstimatedRows)
	infoLabel := canvas.NewText(infoText, color.RGBA{R: 50, G: 50, B: 50, A: 255})
	infoLabel.TextSize = 10
	infoLabel.Move(fyne.NewPos(pos.X+6, pos.Y+28))

	*objects = append(*objects, bg, border, opLabel, infoLabel)

	for _, child := range op.Children {
		drawNodes(child, totalCost, positions, objects)
	}
}

func nodeColor(op *parser.RelOp) color.Color {
	switch op.PhysicalOp {
	case "Table Scan", "Index Scan":
		return color.RGBA{R: 255, G: 120, B: 120, A: 220} // red
	case "Sort", "Spool", "Hash Match":
		return color.RGBA{R: 255, G: 180, B: 80, A: 220} // orange
	case "Index Seek", "Nested Loops", "Clustered Index Seek":
		return color.RGBA{R: 120, G: 220, B: 120, A: 220} // green
	}
	if op.CostPercent > 30 {
		return color.RGBA{R: 255, G: 240, B: 100, A: 220} // yellow
	}
	return color.RGBA{R: 180, G: 210, B: 255, A: 220} // blue/neutral
}

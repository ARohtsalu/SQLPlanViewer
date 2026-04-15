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
	nodeW       = float32(160)
	nodeH       = float32(60)
	nodeSpacingX = float32(200)
	nodeSpacingY = float32(80)
)

// operatorIcon returns a simple unicode icon for the operator.
func operatorIcon(op string) string {
	switch op {
	case "Index Seek", "Clustered Index Seek":
		return "🔍"
	case "Table Scan", "Index Scan", "Clustered Index Scan":
		return "📋"
	case "Nested Loops":
		return "↔"
	case "Hash Match":
		return "#"
	case "Sort":
		return "↕"
	case "Table Spool", "Row Count Spool":
		return "💾"
	default:
		return "▶"
	}
}

// --- Layout ---

type layoutNode struct {
	op *parser.RelOp
	x  float32
	y  float32
}

func layoutTree(op *parser.RelOp, depth int, yOffset *float32) (x, y float32) {
	x = float32(depth) * nodeSpacingX

	if len(op.Children) == 0 {
		y = *yOffset
		*yOffset += nodeSpacingY
		op.X = x
		op.Y = y
		return
	}

	firstY, lastY := float32(0), float32(0)
	for i, child := range op.Children {
		_, cy := layoutTree(child, depth+1, yOffset)
		if i == 0 {
			firstY = cy
		}
		lastY = cy
	}
	y = (firstY + lastY) / 2
	op.X = x
	op.Y = y
	return
}

// --- PlanCanvas widget ---

type PlanCanvas struct {
	widget.BaseWidget
	plan    *parser.QueryPlan
	lang    *Lang
	Scale   float32
	OffsetX float32
	OffsetY float32
}

func newPlanCanvas(plan *parser.QueryPlan, lang *Lang) *PlanCanvas {
	pc := &PlanCanvas{
		plan:  plan,
		lang:  lang,
		Scale: 1.0,
	}
	pc.ExtendBaseWidget(pc)
	return pc
}

func (pc *PlanCanvas) CreateRenderer() fyne.WidgetRenderer {
	return &planRenderer{pc: pc}
}

func (pc *PlanCanvas) Scrolled(ev *fyne.ScrollEvent) {
	if ev.Scrolled.DY > 0 {
		pc.Scale *= 1.1
	} else {
		pc.Scale *= 0.9
	}
	if pc.Scale < 0.1 {
		pc.Scale = 0.1
	}
	if pc.Scale > 5.0 {
		pc.Scale = 5.0
	}
	pc.Refresh()
}

func (pc *PlanCanvas) Dragged(ev *fyne.DragEvent) {
	pc.OffsetX += ev.Dragged.DX
	pc.OffsetY += ev.Dragged.DY
	pc.Refresh()
}

func (pc *PlanCanvas) DragEnd() {}

func (pc *PlanCanvas) MinSize() fyne.Size {
	if pc.plan == nil || pc.plan.RootOp == nil {
		return fyne.NewSize(400, 300)
	}
	w, h := treeSize(pc.plan.RootOp)
	return fyne.NewSize(
		(w+nodeW+40)*pc.Scale,
		(h+nodeH+40)*pc.Scale,
	)
}

func treeSize(op *parser.RelOp) (float32, float32) {
	if op == nil {
		return 0, 0
	}
	maxX := op.X + nodeW
	maxY := op.Y + nodeH
	for _, c := range op.Children {
		cx, cy := treeSize(c)
		if cx > maxX {
			maxX = cx
		}
		if cy > maxY {
			maxY = cy
		}
	}
	return maxX, maxY
}

// --- Renderer ---

type planRenderer struct {
	pc      *PlanCanvas
	objects []fyne.CanvasObject
}

func (r *planRenderer) Layout(_ fyne.Size) {}

func (r *planRenderer) MinSize() fyne.Size {
	return r.pc.MinSize()
}

func (r *planRenderer) Refresh() {
	r.objects = nil
	if r.pc.plan == nil || r.pc.plan.RootOp == nil {
		return
	}
	r.drawEdges(r.pc.plan.RootOp)
	r.drawNodes(r.pc.plan.RootOp)
	canvas.Refresh(r.pc)
}

func (r *planRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *planRenderer) Destroy() {}

func (r *planRenderer) tx(x float32) float32 {
	return x*r.pc.Scale + r.pc.OffsetX + 10
}

func (r *planRenderer) ty(y float32) float32 {
	return y*r.pc.Scale + r.pc.OffsetY + 10
}

func (r *planRenderer) drawEdges(op *parser.RelOp) {
	if op == nil {
		return
	}
	px := r.tx(op.X + nodeW)
	py := r.ty(op.Y + nodeH/2)

	for _, child := range op.Children {
		cx := r.tx(child.X)
		cy := r.ty(child.Y + nodeH/2)

		line := canvas.NewLine(color.RGBA{R: 120, G: 120, B: 120, A: 200})
		line.StrokeWidth = 1.5
		line.Position1 = fyne.NewPos(px, py)
		line.Position2 = fyne.NewPos(cx, cy)
		r.objects = append(r.objects, line)

		r.drawEdges(child)
	}
}

func (r *planRenderer) drawNodes(op *parser.RelOp) {
	if op == nil {
		return
	}

	s := r.pc.Scale
	x := r.tx(op.X)
	y := r.ty(op.Y)
	w := nodeW * s
	h := nodeH * s

	bg := canvas.NewRectangle(graphNodeColor(op))
	bg.CornerRadius = 6 * s
	bg.Resize(fyne.NewSize(w, h))
	bg.Move(fyne.NewPos(x, y))

	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = color.RGBA{R: 60, G: 60, B: 60, A: 180}
	border.StrokeWidth = 1
	border.CornerRadius = 6 * s
	border.Resize(fyne.NewSize(w, h))
	border.Move(fyne.NewPos(x, y))

	icon := operatorIcon(op.PhysicalOp)
	line1 := canvas.NewText(icon+" "+op.PhysicalOp, color.White)
	line1.TextSize = 11 * s
	line1.TextStyle = fyne.TextStyle{Bold: true}
	line1.Move(fyne.NewPos(x+5*s, y+5*s))

	if op.LogicalOp != "" && op.LogicalOp != op.PhysicalOp {
		line2 := canvas.NewText("("+op.LogicalOp+")", color.RGBA{R: 220, G: 220, B: 220, A: 255})
		line2.TextSize = 9 * s
		line2.Move(fyne.NewPos(x+5*s, y+20*s))
	}

	line3 := canvas.NewText(
		fmt.Sprintf("Cost: %.0f%%  |  Rows: %.0f", op.CostPercent, op.EstimatedRows),
		color.RGBA{R: 230, G: 230, B: 230, A: 255},
	)
	line3.TextSize = 9 * s
	line3.Move(fyne.NewPos(x+5*s, y+35*s))

	r.objects = append(r.objects, bg, border, line1, line3)

	for _, child := range op.Children {
		r.drawNodes(child)
	}
}

func graphNodeColor(op *parser.RelOp) color.Color {
	switch op.PhysicalOp {
	case "Table Scan", "Index Scan", "Clustered Index Scan":
		return color.RGBA{R: 200, G: 50, B: 50, A: 255}
	case "Sort", "Hash Match", "Table Spool", "Row Count Spool":
		return color.RGBA{R: 220, G: 130, B: 0, A: 255}
	case "Index Seek", "Clustered Index Seek":
		return color.RGBA{R: 50, G: 150, B: 50, A: 255}
	default:
		if op.CostPercent > 30 {
			return color.RGBA{R: 220, G: 200, B: 0, A: 255}
		}
		return color.RGBA{R: 70, G: 130, B: 180, A: 255}
	}
}

// --- PlanGraph: public entry point ---

type PlanGraph struct {
	plan *parser.QueryPlan
	lang *Lang
}

func NewPlanGraph(plan *parser.QueryPlan, lang *Lang) *PlanGraph {
	return &PlanGraph{plan: plan, lang: lang}
}

func (pg *PlanGraph) Widget() fyne.CanvasObject {
	if pg.plan == nil || pg.plan.RootOp == nil {
		return widget.NewLabel("(no plan graph)")
	}

	// Layout the tree
	yOffset := float32(10)
	layoutTree(pg.plan.RootOp, 0, &yOffset)

	pc := newPlanCanvas(pg.plan, pg.lang)

	zoomIn := widget.NewButton("+", func() {
		pc.Scale *= 1.2
		if pc.Scale > 5.0 {
			pc.Scale = 5.0
		}
		pc.Refresh()
	})
	zoomOut := widget.NewButton("-", func() {
		pc.Scale *= 0.8
		if pc.Scale < 0.1 {
			pc.Scale = 0.1
		}
		pc.Refresh()
	})
	reset := widget.NewButton("Reset", func() {
		pc.Scale = 1.0
		pc.OffsetX = 0
		pc.OffsetY = 0
		pc.Refresh()
	})

	controls := container.NewHBox(zoomIn, zoomOut, reset)
	scroll := container.NewScroll(pc)

	return container.NewBorder(controls, nil, nil, nil, scroll)
}

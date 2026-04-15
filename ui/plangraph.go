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
	nodeW        = float32(160)
	nodeH        = float32(60)
	nodeSpacingX = float32(200)
	nodeSpacingY = float32(80)
)

func operatorIcon(op string) string {
	switch op {
	case "Index Seek", "Clustered Index Seek":
		return "🔍"
	case "Index Scan", "Clustered Index Scan":
		return "📋"
	case "Table Scan":
		return "⚠"
	case "Hash Match":
		return "#"
	case "Nested Loops":
		return "↩"
	case "Sort":
		return "↕"
	case "Table Spool", "Row Count Spool":
		return "💾"
	case "Table Insert", "Clustered Index Insert":
		return "+"
	case "Compute Scalar":
		return "∑"
	case "Filter":
		return "▽"
	case "Top":
		return "⊤"
	case "Stream Aggregate":
		return "≡"
	case "Key Lookup":
		return "🔑"
	default:
		return "▸"
	}
}

func formatRows(rows float64) string {
	if rows >= 1_000_000 {
		return fmt.Sprintf("%.1fM", rows/1_000_000)
	}
	if rows >= 1_000 {
		return fmt.Sprintf("%.0fK", rows/1_000)
	}
	return fmt.Sprintf("%.0f", rows)
}

// layoutTree assigns X,Y to every node. Returns y position of this node.
func layoutTree(op *parser.RelOp, depth int, yOffset *float32) float32 {
	if len(op.Children) == 0 {
		op.X = float32(depth) * nodeSpacingX
		op.Y = *yOffset
		*yOffset += nodeSpacingY
		return op.Y
	}
	firstY, lastY := float32(0), float32(0)
	for i, child := range op.Children {
		cy := layoutTree(child, depth+1, yOffset)
		if i == 0 {
			firstY = cy
		}
		lastY = cy
	}
	op.X = float32(depth) * nodeSpacingX
	op.Y = (firstY + lastY) / 2
	return op.Y
}

func calcTreeSize(op *parser.RelOp) (w, h float32) {
	if op == nil {
		return 0, 0
	}
	maxX := op.X + nodeW
	maxY := op.Y + nodeH
	for _, c := range op.Children {
		cw, ch := calcTreeSize(c)
		if cw > maxX {
			maxX = cw
		}
		if ch > maxY {
			maxY = ch
		}
	}
	return maxX, maxY
}

// PlanCanvas is a zoomable/pannable canvas for the plan graph.
type PlanCanvas struct {
	widget.BaseWidget
	plan       *parser.QueryPlan
	lang       *Lang
	Scale      float32
	OffsetX    float32
	OffsetY    float32
	treeWidth  float32
	treeHeight float32
}

func newPlanCanvas(plan *parser.QueryPlan, lang *Lang, initialScale float32) *PlanCanvas {
	pc := &PlanCanvas{plan: plan, lang: lang, Scale: initialScale}
	if plan != nil && plan.RootOp != nil {
		pc.treeWidth, pc.treeHeight = calcTreeSize(plan.RootOp)
		pc.treeWidth += 40
		pc.treeHeight += 40
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
	return fyne.NewSize(pc.treeWidth*pc.Scale, pc.treeHeight*pc.Scale)
}

type planRenderer struct {
	pc      *PlanCanvas
	objects []fyne.CanvasObject
}

func (r *planRenderer) Layout(_ fyne.Size) {}
func (r *planRenderer) MinSize() fyne.Size { return r.pc.MinSize() }
func (r *planRenderer) Destroy()           {}
func (r *planRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *planRenderer) Refresh() {
	r.objects = nil
	if r.pc.plan == nil || r.pc.plan.RootOp == nil {
		canvas.Refresh(r.pc)
		return
	}
	r.drawEdges(r.pc.plan.RootOp)
	r.drawNodes(r.pc.plan.RootOp)
	canvas.Refresh(r.pc)
}

func (r *planRenderer) tx(x float32) float32 { return x*r.pc.Scale + r.pc.OffsetX + 10 }
func (r *planRenderer) ty(y float32) float32 { return y*r.pc.Scale + r.pc.OffsetY + 10 }

func (r *planRenderer) drawEdges(op *parser.RelOp) {
	s := r.pc.Scale
	px := r.tx(op.X + nodeW)
	py := r.ty(op.Y + nodeH/2)
	for _, child := range op.Children {
		cx := r.tx(child.X)
		cy := r.ty(child.Y + nodeH/2)
		line := canvas.NewLine(color.RGBA{R: 100, G: 100, B: 100, A: 200})
		line.StrokeWidth = 1.5 * s
		line.Position1 = fyne.NewPos(px, py)
		line.Position2 = fyne.NewPos(cx, cy)
		r.objects = append(r.objects, line)
		r.drawEdges(child)
	}
}

func (r *planRenderer) drawNodes(op *parser.RelOp) {
	s := r.pc.Scale
	x := r.tx(op.X)
	y := r.ty(op.Y)
	w := nodeW * s
	h := nodeH * s

	bg := canvas.NewRectangle(graphNodeColor(op))
	bg.CornerRadius = 4 * s
	bg.Resize(fyne.NewSize(w, h))
	bg.Move(fyne.NewPos(x, y))

	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = color.RGBA{R: 40, G: 40, B: 40, A: 180}
	border.StrokeWidth = 1
	border.CornerRadius = 4 * s
	border.Resize(fyne.NewSize(w, h))
	border.Move(fyne.NewPos(x, y))

	// Row 1: icon + PhysicalOp (bold)
	icon := canvas.NewText(operatorIcon(op.PhysicalOp), color.White)
	icon.TextSize = 11 * s
	icon.Move(fyne.NewPos(x+4*s, y+4*s))

	opName := canvas.NewText(op.PhysicalOp, color.White)
	opName.TextSize = 11 * s
	opName.TextStyle = fyne.TextStyle{Bold: true}
	opName.Move(fyne.NewPos(x+20*s, y+4*s))

	// Row 2: LogicalOp (italic, lighter)
	r.objects = append(r.objects, bg, border, icon, opName)
	if op.LogicalOp != "" && op.LogicalOp != op.PhysicalOp {
		logical := canvas.NewText("("+op.LogicalOp+")", color.RGBA{R: 230, G: 230, B: 230, A: 255})
		logical.TextSize = 10 * s
		logical.Move(fyne.NewPos(x+20*s, y+18*s))
		r.objects = append(r.objects, logical)
	}

	// Row 3: Cost% + Rows (bottom)
	stats := canvas.NewText(
		fmt.Sprintf("Cost: %.0f%%  Rows: %s", op.CostPercent, formatRows(op.EstimatedRows)),
		color.White)
	stats.TextSize = 9 * s
	stats.Move(fyne.NewPos(x+4*s, y+h-14*s))
	r.objects = append(r.objects, stats)

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
			return color.RGBA{R: 200, G: 180, B: 0, A: 255}
		}
		return color.RGBA{R: 70, G: 130, B: 180, A: 255}
	}
}

// PlanGraph is the public entry point for a single statement graph.
type PlanGraph struct {
	plan *parser.QueryPlan
	lang *Lang
}

func NewPlanGraph(plan *parser.QueryPlan, lang *Lang) *PlanGraph {
	return &PlanGraph{plan: plan, lang: lang}
}

func (pg *PlanGraph) Widget() fyne.CanvasObject {
	return buildPlanCanvasWidget(pg.plan, pg.lang, 1.0)
}

// MiniWidget returns a smaller, scrollable plan canvas for overview use.
func (pg *PlanGraph) MiniWidget() fyne.CanvasObject {
	return buildPlanCanvasWidget(pg.plan, pg.lang, 0.55)
}

func buildPlanCanvasWidget(plan *parser.QueryPlan, lang *Lang, scale float32) fyne.CanvasObject {
	if plan == nil || plan.RootOp == nil {
		return widget.NewLabel("(no plan graph)")
	}

	yOff := float32(10)
	layoutTree(plan.RootOp, 0, &yOff)

	pc := newPlanCanvas(plan, lang, scale)

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
		pc.Scale = scale
		pc.OffsetX = 0
		pc.OffsetY = 0
		pc.Refresh()
	})

	controls := container.NewHBox(zoomIn, zoomOut, reset)
	scroll := container.NewScroll(pc)
	scroll.SetMinSize(fyne.NewSize(400, 200))

	return container.NewBorder(controls, nil, nil, nil, scroll)
}

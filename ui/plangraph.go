package ui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"sqlplanviewer/parser"
)

// Layout constants — ported from PlanLayoutEngine.cs
const (
	nodeW      = float32(150)
	hSpacing   = float32(180) // horizontal gap between depth levels
	vSpacing   = float32(24)  // vertical gap between leaf siblings
	layoutPad  = float32(40)  // initial X and Y padding

	// Row heights for node height calculation (from PlanLayoutEngine.cs comments)
	iconRowH = float32(36) // 32px icon + margin + spacing
	line10   = float32(17) // FontSize-10 text row
	line9    = float32(15) // FontSize-9 text row
	nodePad  = float32(12) // border padding top+bottom
	nodeMinH = float32(90) // minimum node height
)

// Colors — ported from DarkTheme.axaml and PlanViewerControl.axaml.cs brushes
var (
	colNodeBg    = color.RGBA{R: 0x22, G: 0x25, B: 0x2D, A: 255} // base dark background
	colNodeBorder = color.RGBA{R: 0x3A, G: 0x3D, B: 0x45, A: 255} // standard border

	// Three-tier cost coloring: subtle yellow → amber → matte red.
	// Alpha values keep the overlays readable on the dark base.
	colNodeBgLow  = color.RGBA{R: 0xF5, G: 0xC8, B: 0x42, A: 0x18} // 0–10 %: warm yellow tint
	colNodeBgMid  = color.RGBA{R: 0xE5, G: 0x90, B: 0x30, A: 0x2A} // 10–25%: amber tint
	colNodeBgExp  = color.RGBA{R: 0xC0, G: 0x40, B: 0x40, A: 0x3C} // ≥25 %: matte red tint
	colNodeBorMid = color.RGBA{R: 0xC8, G: 0x88, B: 0x30, A: 255}  // 10–25%: amber border
	colNodeBorExp = color.RGBA{R: 0xB0, G: 0x30, B: 0x30, A: 255}  // ≥25 %: dark matte red border

	colText          = color.RGBA{R: 0xE4, G: 0xE6, B: 0xEB, A: 255} // ForegroundBrush
	colCostOrange    = color.RGBA{R: 0xFF, G: 0xA5, B: 0x00, A: 255} // Orange  (≥25%)
	colCostOrangeRed = color.RGBA{R: 0xFF, G: 0x45, B: 0x00, A: 255} // OrangeRed (≥50%)
	colEdge          = color.RGBA{R: 0x6B, G: 0x72, B: 0x80, A: 200} // EdgeBrush
)

// nodeHeight calculates the rendered height of a node.
// Must exactly match the drawNodes layout so no whitespace gap appears at bottom.
//
// drawNodes vertical layout (logical px):
//   y+4  : icon top  (32px tall)
//   y+38 : operator name (line10 = 17px)
//   y+55 : cost % row   (line10 = 17px)
//   y+72 : rows row      (line9  = 15px)
//   y+87 : obj name row  (line9  = 15px, optional)
//   y+102 total with obj, y+87 without
// Add 8px bottom pad → 95 without obj, 110 with obj.
func nodeHeight(op *parser.RelOp) float32 {
	h := float32(95) // top-margin(4)+icon(32)+gap(2)+name(17)+cost(17)+rows(15)+bottom-pad(8)
	if op.ObjTable != "" {
		h += line9 // one line for schema.table[.index]
	}
	if h < nodeMinH {
		h = nodeMinH
	}
	return h
}

// ── Layout ──────────────────────────────────────────────────────────────────

// layoutTree lays out the tree in Erik's LEFT-RIGHT style:
//   Phase 1 — X = padding + depth * hSpacing  (root at left, children to the right)
//   Phase 2 — leaves placed top-to-bottom; parent Y = first child's Y (SSMS style)
func layoutTree(op *parser.RelOp) {
	cacheHeights(op)
	setXPositions(op, 0)
	nextY := layoutPad
	setYPositions(op, &nextY)
}

// cacheHeights pre-computes and stores nodeHeight in each RelOp,
// avoiding thousands of redundant recalculations during draw/hit-test.
func cacheHeights(op *parser.RelOp) {
	op.CachedHeight = nodeHeight(op)
	for _, c := range op.Children {
		cacheHeights(c)
	}
}

func setXPositions(op *parser.RelOp, depth int) {
	op.X = layoutPad + float32(depth)*hSpacing
	for _, c := range op.Children {
		setXPositions(c, depth+1)
	}
}

func setYPositions(op *parser.RelOp, nextY *float32) {
	if len(op.Children) == 0 {
		op.Y = *nextY
		*nextY += op.CachedHeight + vSpacing
		return
	}
	// Post-order: process children first
	for _, c := range op.Children {
		setYPositions(c, nextY)
	}
	// SSMS style: parent aligns with first (topmost) child
	op.Y = op.Children[0].Y
}

func calcTreeSize(op *parser.RelOp) (w, h float32) {
	if op == nil {
		return 0, 0
	}
	maxX := op.X + nodeW
	maxY := op.Y + op.CachedHeight
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

func collectAllNodes(root *parser.RelOp) []*parser.RelOp {
	if root == nil {
		return nil
	}
	result := []*parser.RelOp{root}
	for _, child := range root.Children {
		result = append(result, collectAllNodes(child)...)
	}
	return result
}

// ── PlanCanvas ───────────────────────────────────────────────────────────────

type PlanCanvas struct {
	widget.BaseWidget
	plan        *parser.QueryPlan
	lang        *Lang
	Scale       float32
	OffsetX     float32
	OffsetY     float32
	treeWidth   float32
	treeHeight  float32
	Interactive bool
	win         fyne.Window
	fitted      bool       // auto-fit done once; subsequent resizes don't rescale
	fittedSize  fyne.Size  // the size at which we last auto-fitted

	allNodes    []*parser.RelOp
	hoveredNode *parser.RelOp
	pinnedNode  *parser.RelOp   // non-nil when tooltip is click-pinned
	tooltipNode *parser.RelOp   // node to draw tooltip for (hover or pinned)
	tooltipPos  fyne.Position   // widget-relative anchor position for tooltip
}

func newPlanCanvas(plan *parser.QueryPlan, lang *Lang, initialScale float32, win fyne.Window) *PlanCanvas {
	pc := &PlanCanvas{
		plan:        plan,
		lang:        lang,
		Scale:       initialScale,
		win:         win,
		Interactive: win != nil,
	}
	if plan != nil && plan.RootOp != nil {
		pc.treeWidth, pc.treeHeight = calcTreeSize(plan.RootOp)
		pc.treeWidth += layoutPad
		pc.treeHeight += layoutPad
		pc.allNodes = collectAllNodes(plan.RootOp)
	}
	pc.ExtendBaseWidget(pc)
	return pc
}

func (pc *PlanCanvas) CreateRenderer() fyne.WidgetRenderer {
	return &planRenderer{pc: pc}
}

func (pc *PlanCanvas) FitToWindow(availW, availH float32) {
	if pc.treeWidth == 0 || pc.treeHeight == 0 {
		return
	}
	// Ignore sizes that arrive before the real layout pass (e.g. MinSize probes).
	if availW < 100 || availH < 100 {
		return
	}
	scaleX := availW / (pc.treeWidth + 40)
	scaleY := availH / (pc.treeHeight + 40)
	pc.Scale = scaleX
	if scaleY < scaleX {
		pc.Scale = scaleY
	}
	if pc.Scale < 0.15 {
		pc.Scale = 0.15
	}
	// Interactive canvas: allow zoom beyond 1.0 so small plans fill the window.
	// Mini preview canvas (Interactive=false): keep native scale, never zoom in.
	maxScale := float32(3.0)
	if !pc.Interactive {
		maxScale = 1.0
	}
	if pc.Scale > maxScale {
		pc.Scale = maxScale
	}
	pc.OffsetX = (availW - pc.treeWidth*pc.Scale) / 2
	if pc.OffsetX < 10 {
		pc.OffsetX = 10
	}
	pc.OffsetY = 10
}

func (pc *PlanCanvas) Resize(size fyne.Size) {
	pc.BaseWidget.Resize(size)
	if !pc.fitted {
		// Wait for a real layout size before fitting — Fyne probes with MinSize
		// (100×100) before the actual allocation arrives. Locking in on a small
		// size would leave the plan zoomed out and never auto-corrected.
		if size.Width >= 200 && size.Height >= 200 {
			pc.fitted = true
			pc.fittedSize = size
			pc.FitToWindow(size.Width, size.Height)
		}
		return
	}
	// Refit when the window is resized significantly (e.g. moved to another monitor).
	// Threshold: >25% change in either dimension.
	if pc.Interactive && pc.fittedSize.Width > 0 {
		dw := abs32(size.Width-pc.fittedSize.Width) / pc.fittedSize.Width
		dh := abs32(size.Height-pc.fittedSize.Height) / pc.fittedSize.Height
		if dw > 0.25 || dh > 0.25 {
			pc.fittedSize = size
			pc.FitToWindow(size.Width, size.Height)
		}
	}
	pc.Refresh()
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
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
	return fyne.NewSize(100, 100)
}

// desktop.Hoverable
func (pc *PlanCanvas) MouseIn(_ *desktop.MouseEvent) {}
func (pc *PlanCanvas) MouseOut()                      {}

func (pc *PlanCanvas) MouseMoved(ev *desktop.MouseEvent) {
	if !pc.Interactive {
		return
	}
	node := pc.nodeAtPos(ev.Position)

	if node == nil {
		pc.hoveredNode = nil
		if pc.pinnedNode == nil && pc.tooltipNode != nil {
			pc.tooltipNode = nil
			pc.Refresh()
		}
		return
	}

	if node == pc.hoveredNode {
		return
	}
	pc.hoveredNode = node

	if pc.pinnedNode != nil && node == pc.pinnedNode {
		return // back on pinned node — tooltip already visible
	}

	// New node: drop pin, show hover tooltip.
	pc.pinnedNode = nil
	pc.tooltipNode = node
	pc.tooltipPos = ev.Position
	pc.Refresh()
}

// Tapped pins/unpins the tooltip on click.
func (pc *PlanCanvas) Tapped(ev *fyne.PointEvent) {
	if !pc.Interactive {
		return
	}
	node := pc.nodeAtPos(ev.Position)

	if node == nil || node == pc.pinnedNode {
		pc.pinnedNode = nil
		pc.tooltipNode = nil
		pc.Refresh()
		return
	}

	pc.pinnedNode = node
	pc.tooltipNode = node
	pc.tooltipPos = ev.Position
	pc.Refresh()
}

func (pc *PlanCanvas) nodeAtPos(pos fyne.Position) *parser.RelOp {
	for _, node := range pc.allNodes {
		nx := node.X*pc.Scale + pc.OffsetX
		ny := node.Y*pc.Scale + pc.OffsetY
		nw := nodeW * pc.Scale
		nh := node.CachedHeight * pc.Scale
		if pos.X >= nx && pos.X <= nx+nw &&
			pos.Y >= ny && pos.Y <= ny+nh {
			return node
		}
	}
	return nil
}

// ── Renderer ─────────────────────────────────────────────────────────────────

type planRenderer struct {
	pc      *PlanCanvas
	objects []fyne.CanvasObject
}

func (r *planRenderer) Layout(_ fyne.Size) {}
func (r *planRenderer) MinSize() fyne.Size { return r.pc.MinSize() }
func (r *planRenderer) Destroy()           {}
func (r *planRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *planRenderer) Refresh() {
	r.objects = r.objects[:0] // reuse backing array, avoid GC
	if r.pc.plan == nil || r.pc.plan.RootOp == nil {
		canvas.Refresh(r.pc)
		return
	}
	r.drawEdges(r.pc.plan.RootOp)
	r.drawNodes(r.pc.plan.RootOp)
	r.drawTooltip()
	canvas.Refresh(r.pc)
}

// drawTooltip renders the tooltip as plain canvas objects — these do not
// intercept mouse events, so hover/click detection on PlanCanvas is unaffected.
//
// Layout: fixed-width (420px) panel with section headers (blue) and
// two-column label+value rows. Long values wrap automatically.
func (r *planRenderer) drawTooltip() {
	node := r.pc.tooltipNode
	if node == nil {
		return
	}
	pos := r.pc.tooltipPos

	const (
		tipW        = float32(420)
		pad         = float32(10)
		labelW      = float32(130) // label column width
		lineH       = float32(17)
		sectionH    = float32(19)
		titleH      = float32(22)
		maxValChars = 42 // approx chars before wrapping in value column
	)

	colBlue  := color.RGBA{R: 0x4F, G: 0xA3, B: 0xFF, A: 255}
	colLabel := color.RGBA{R: 0x9C, G: 0x9F, B: 0xA8, A: 255}
	colCode  := color.RGBA{R: 0xC8, G: 0xE6, B: 0xFF, A: 255}
	colObj   := color.RGBA{R: 0x88, G: 0xBB, B: 0xFF, A: 255}

	// ── Entry list ────────────────────────────────────────────────────────────
	type entry struct {
		kind   int    // 0=section, 1=kv, 2=warn
		label  string // section title / kv label / warn text
		value  string
		col    color.Color
		vlines []string // pre-wrapped value lines
	}

	var entries []entry
	section := func(title string) {
		entries = append(entries, entry{kind: 0, label: title, col: colBlue})
	}
	kv := func(label, value string, col color.Color) {
		entries = append(entries, entry{1, label, value, col, wrapText(value, maxValChars)})
	}
	warn := func(text string) {
		entries = append(entries, entry{kind: 2, label: text, col: colCostOrange})
	}

	// Costs
	section("Costs")
	costCol := colText
	if node.CostPercent >= 50 {
		costCol = colCostOrangeRed
	} else if node.CostPercent >= 25 {
		costCol = colCostOrange
	}
	ownCost := node.EstimateIO + node.EstimateCPU
	if ownCost > 0 {
		kv("Cost", fmt.Sprintf("%.0f%%  (%.6f)", node.CostPercent, ownCost), costCol)
	} else {
		kv("Cost", fmt.Sprintf("%.0f%%", node.CostPercent), costCol)
	}
	kv("Subtree Cost", fmt.Sprintf("%.6f", node.EstimatedCost), colText)

	// Rows
	section("Rows")
	kv("Estimated Rows", formatRows(node.EstimatedRows), colText)
	if node.EstRowsRead > 0 && node.EstRowsRead != node.EstimatedRows {
		kv("Est. Rows Read", formatRows(node.EstRowsRead), colText)
	}

	// Estimates (I/O, CPU, sizes)
	if node.EstimateIO > 0 || node.EstimateCPU > 0 || node.AvgRowSize > 0 || node.TableCardinality > 0 {
		section("Estimates")
		if node.EstimateIO > 0 {
			kv("I/O Cost", fmt.Sprintf("%.6f", node.EstimateIO), colText)
		}
		if node.EstimateCPU > 0 {
			kv("CPU Cost", fmt.Sprintf("%.6f", node.EstimateCPU), colText)
		}
		if node.AvgRowSize > 0 {
			kv("Avg Row Size", fmt.Sprintf("%.0f B", node.AvgRowSize), colText)
		}
		if node.TableCardinality > 0 {
			kv("Table Cardinality", formatRows(node.TableCardinality), colText)
		}
	}

	// Rebinds / Rewinds
	if node.EstRebinds > 0 || node.EstRewinds > 0 {
		section("Rebinds / Rewinds")
		kv("Est. Rebinds", fmt.Sprintf("%.1f", node.EstRebinds), colText)
		kv("Est. Rewinds", fmt.Sprintf("%.1f", node.EstRewinds), colText)
	}

	// Parallelism
	if node.Parallel || node.ExecutionMode != "" {
		section("Parallelism")
		if node.Parallel {
			kv("Parallel", "Yes", colText)
		}
		if node.ExecutionMode != "" {
			kv("Execution Mode", node.ExecutionMode, colText)
		}
	}

	// Object
	if node.ObjTable != "" {
		section("Object")
		objName := node.ObjSchema + "." + node.ObjTable
		if node.ObjIndex != "" {
			objName += "." + node.ObjIndex
		}
		kv("Name", objName, colObj)
		if node.ObjIndexKind != "" {
			kv("Index Kind", node.ObjIndexKind, colText)
		}
		if node.ObjStorage != "" {
			kv("Storage", node.ObjStorage, colText)
		}
	}

	// Predicates
	if node.SeekPredicate != "" || node.Predicate != "" {
		section("Predicates")
		if node.SeekPredicate != "" {
			kv("Seek", node.SeekPredicate, colCode)
		}
		if node.Predicate != "" {
			kv("Residual", node.Predicate, colCode)
		}
	}

	// Output columns
	if len(node.OutputColNames) > 0 {
		section("Output")
		cols := strings.Join(node.OutputColNames, ", ")
		const maxShow = 10
		if len(node.OutputColNames) > maxShow {
			cols = strings.Join(node.OutputColNames[:maxShow], ", ") +
				fmt.Sprintf("  (+%d)", len(node.OutputColNames)-maxShow)
		}
		kv("Columns", cols, colLabel)
	}

	// Warnings
	for _, w := range node.WarningTexts {
		warn("⚠ " + w)
	}

	// ── Calculate total height ────────────────────────────────────────────────
	totalH := pad + titleH + float32(4)
	for _, e := range entries {
		switch e.kind {
		case 0:
			totalH += sectionH + 3
		case 1:
			n := len(e.vlines)
			if n == 0 {
				n = 1
			}
			totalH += lineH*float32(n) + 2
		case 2:
			totalH += lineH + 2
		}
	}
	totalH += pad

	// ── Position — flip left/up if near canvas edge ──────────────────────────
	tipX := pos.X + 16
	tipY := pos.Y + 10
	sz := r.pc.Size()
	if tipX+tipW+8 > sz.Width {
		tipX = pos.X - tipW - 12
		if tipX < 4 {
			tipX = 4
		}
	}
	if tipY+totalH+8 > sz.Height {
		tipY = sz.Height - totalH - 8
		if tipY < 4 {
			tipY = 4
		}
	}

	// ── Background & border ──────────────────────────────────────────────────
	bg := canvas.NewRectangle(color.RGBA{R: 0x1A, G: 0x1D, B: 0x25, A: 250})
	bg.Move(fyne.NewPos(tipX, tipY))
	bg.Resize(fyne.NewSize(tipW, totalH))
	bord := canvas.NewRectangle(color.Transparent)
	bord.StrokeColor = colNodeBorder
	bord.StrokeWidth = 1
	bord.Move(fyne.NewPos(tipX, tipY))
	bord.Resize(fyne.NewSize(tipW, totalH))
	r.objects = append(r.objects, bg, bord)

	addText := func(text string, x, y float32, col color.Color, bold bool, size float32) {
		t := canvas.NewText(text, col)
		t.TextStyle = fyne.TextStyle{Bold: bold}
		t.TextSize = size
		t.Move(fyne.NewPos(x, y))
		r.objects = append(r.objects, t)
	}

	// ── Title ────────────────────────────────────────────────────────────────
	title := node.PhysicalOp
	if node.LogicalOp != "" && node.LogicalOp != node.PhysicalOp {
		title += "  (" + node.LogicalOp + ")"
	}
	addText(title, tipX+pad, tipY+pad, colText, true, 12)

	// ── Entries ──────────────────────────────────────────────────────────────
	curY := tipY + pad + titleH + 4

	for _, e := range entries {
		switch e.kind {
		case 0: // section header
			addText(e.label, tipX+pad, curY, e.col, true, 9)
			curY += sectionH + 3
		case 1: // label + value(s)
			addText(e.label+":", tipX+pad, curY, colLabel, false, 9)
			vlines := e.vlines
			if len(vlines) == 0 {
				vlines = []string{e.value}
			}
			for i, vl := range vlines {
				addText(vl, tipX+labelW, curY+float32(i)*lineH, e.col, false, 10)
			}
			curY += lineH*float32(len(vlines)) + 2
		case 2: // warning
			addText(e.label, tipX+pad, curY, e.col, false, 10)
			curY += lineH + 2
		}
	}
}

// tx/ty: logical → screen coordinates
func (r *planRenderer) tx(x float32) float32 { return x*r.pc.Scale + r.pc.OffsetX }
func (r *planRenderer) ty(y float32) float32 { return y*r.pc.Scale + r.pc.OffsetY }

// drawEdges draws L-shaped elbow connectors.
// Parent right-center → midX → child left-center.
// Thickness: log-based on estimated rows (matches Erik's formula).
func (r *planRenderer) drawEdges(op *parser.RelOp) {
	s := r.pc.Scale
	parentRight := r.tx(op.X + nodeW)
	parentCenterY := r.ty(op.Y + op.CachedHeight/2)

	for _, child := range op.Children {
		ch := child.CachedHeight
		childLeft := r.tx(child.X)
		childCenterY := r.ty(child.Y + ch/2)
		midX := (parentRight + childLeft) / 2

		// Log-based thickness: max(2, min(floor(log(max(1, rows))), 12))
		rows := child.EstimatedRows
		if rows < 1 {
			rows = 1
		}
		thickness := float32(math.Max(2, math.Min(math.Floor(math.Log(rows)), 12))) * s

		addLine := func(x1, y1, x2, y2 float32) {
			l := canvas.NewLine(colEdge)
			l.StrokeWidth = thickness
			l.Position1 = fyne.NewPos(x1, y1)
			l.Position2 = fyne.NewPos(x2, y2)
			r.objects = append(r.objects, l)
		}

		// Segment 1: parent right → midX (horizontal)
		addLine(parentRight, parentCenterY, midX, parentCenterY)
		// Segment 2: midX, parentY → midX, childY (vertical)
		addLine(midX, parentCenterY, midX, childCenterY)
		// Segment 3: midX → child left (horizontal)
		addLine(midX, childCenterY, childLeft, childCenterY)

		// Arrowhead at parent connection: tip at parentRight, wings open right →
		// Indicates data flows from child (right) into parent (left), SSMS convention.
		aLen := 6 * s
		aHalf := 3 * s
		addLine(parentRight, parentCenterY, parentRight+aLen, parentCenterY-aHalf)
		addLine(parentRight, parentCenterY, parentRight+aLen, parentCenterY+aHalf)

		r.drawEdges(child)
	}
}

// drawNodes draws each node box with PNG icon, operator name, cost%, and object name.
func (r *planRenderer) drawNodes(op *parser.RelOp) {
	s := r.pc.Scale
	x := r.tx(op.X)
	y := r.ty(op.Y)
	w := nodeW * s
	h := op.CachedHeight * s

	// Three-tier cost coloring
	var tintColor, borderColor color.Color
	var borderW float32
	switch {
	case op.CostPercent >= 25:
		tintColor, borderColor, borderW = colNodeBgExp, colNodeBorExp, 2
	case op.CostPercent >= 10:
		tintColor, borderColor, borderW = colNodeBgMid, colNodeBorMid, 1
	case op.CostPercent > 0:
		tintColor, borderColor, borderW = colNodeBgLow, colNodeBorder, 1
	default:
		tintColor, borderColor, borderW = color.Transparent, colNodeBorder, 1
	}

	// Base background (always opaque dark)
	bg := canvas.NewRectangle(colNodeBg)
	bg.CornerRadius = 4 * s
	bg.Resize(fyne.NewSize(w, h))
	bg.Move(fyne.NewPos(x, y))
	r.objects = append(r.objects, bg)

	// Cost tint overlay (semi-transparent, blends over base)
	if tintColor != color.Transparent {
		tint := canvas.NewRectangle(tintColor)
		tint.CornerRadius = 4 * s
		tint.Resize(fyne.NewSize(w, h))
		tint.Move(fyne.NewPos(x, y))
		r.objects = append(r.objects, tint)
	}

	// Border
	bord := canvas.NewRectangle(color.Transparent)
	bord.StrokeColor = borderColor
	bord.StrokeWidth = borderW
	bord.CornerRadius = 4 * s
	bord.Resize(fyne.NewSize(w, h))
	bord.Move(fyne.NewPos(x, y))

	r.objects = append(r.objects, bord)

	// Icon (32x32, centered horizontally, 4px top margin)
	iconSize := float32(32) * s
	iconX := x + (w-iconSize)/2
	iconY := y + 4*s
	iconName := planIconName(op.PhysicalOp)
	if res := loadIconResource(iconName); res != nil {
		img := canvas.NewImageFromResource(res)
		img.FillMode = canvas.ImageFillContain
		img.Resize(fyne.NewSize(iconSize, iconSize))
		img.Move(fyne.NewPos(iconX, iconY))
		r.objects = append(r.objects, img)
	}

	// Warning badge — always visible (important signal even when zoomed out).
	// Size clamped so it stays readable at low scale.
	if op.HasWarnings {
		badgeSize := 12 * s
		if badgeSize < 9 {
			badgeSize = 9
		}
		badge := canvas.NewText("⚠", color.RGBA{R: 0xFF, G: 0xA5, B: 0x00, A: 255})
		badge.TextSize = badgeSize
		badge.Move(fyne.NewPos(x+w-16*s, y+4*s))
		r.objects = append(r.objects, badge)
	}

	// Parallel badge — same clamped sizing.
	if op.Parallel {
		parSize := 11 * s
		if parSize < 9 {
			parSize = 9
		}
		par := canvas.NewText("⇆", color.RGBA{R: 0xFF, G: 0xC1, B: 0x07, A: 255})
		par.TextSize = parSize
		par.Move(fyne.NewPos(x+w-28*s, y+4*s))
		r.objects = append(r.objects, par)
	}

	// Text labels: only rendered when zoomed in enough to be legible.
	// Below the threshold the node is too small to read — icons + colors give the overview.
	const textShowScale = float32(0.40)
	if s >= textShowScale {
		// Clamp text sizes to a minimum so they stay readable near the threshold.
		nameSize := 10 * s
		if nameSize < 7 {
			nameSize = 7
		}
		smallSize := 9 * s
		if smallSize < 7 {
			smallSize = 7
		}

		// Operator name
		nameY := iconY + iconSize + 2*s
		opName := canvas.NewText(op.PhysicalOp, colText)
		opName.TextSize = nameSize
		opName.TextStyle = fyne.TextStyle{Bold: true}
		opName.Move(fyne.NewPos(x+8*s, nameY))
		r.objects = append(r.objects, opName)

		// Cost% row
		costY := nameY + line10*s
		var costColor color.Color
		switch {
		case op.CostPercent >= 50:
			costColor = colCostOrangeRed
		case op.CostPercent >= 25:
			costColor = colCostOrange
		default:
			costColor = colText
		}
		costTxt := canvas.NewText(fmt.Sprintf("Cost: %d%%", int(op.CostPercent)), costColor)
		costTxt.TextSize = nameSize
		costTxt.Move(fyne.NewPos(x+8*s, costY))
		r.objects = append(r.objects, costTxt)

		// Est rows
		rowsY := costY + line10*s
		rowsTxt := canvas.NewText(fmt.Sprintf("Rows: %s", formatRows(op.EstimatedRows)), colText)
		rowsTxt.TextSize = smallSize
		rowsTxt.Move(fyne.NewPos(x+8*s, rowsY))
		r.objects = append(r.objects, rowsTxt)

		// Object name — schema.table or schema.table.index (one line, truncated)
		if op.ObjTable != "" {
			objText := op.ObjSchema + "." + op.ObjTable
			if op.ObjIndex != "" {
				objText += "." + op.ObjIndex
			}
			objY := rowsY + line9*s
			avail := float32(nodeW) - 16
			maxChars := int(float64(avail) / 5.5)
			if len(objText) > maxChars {
				objText = objText[:maxChars-2] + ".."
			}
			objLabel := canvas.NewText(objText, colText)
			objLabel.TextSize = smallSize
			objLabel.Move(fyne.NewPos(x+8*s, objY))
			r.objects = append(r.objects, objLabel)
		}
	}

	for _, child := range op.Children {
		r.drawNodes(child)
	}
}

// wrapText splits s into lines no longer than maxChars, breaking at spaces.
func wrapText(s string, maxChars int) []string {
	if len(s) <= maxChars {
		return []string{s}
	}
	var lines []string
	for len(s) > maxChars {
		cut := maxChars
		if i := strings.LastIndexByte(s[:maxChars], ' '); i > maxChars/3 {
			cut = i
		}
		lines = append(lines, s[:cut])
		s = strings.TrimLeft(s[cut:], " ")
	}
	if s != "" {
		lines = append(lines, s)
	}
	return lines
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

// ── Public API ───────────────────────────────────────────────────────────────

type PlanGraph struct {
	plan *parser.QueryPlan
	lang *Lang
	win  fyne.Window
}

func NewPlanGraph(plan *parser.QueryPlan, lang *Lang, win fyne.Window) *PlanGraph {
	return &PlanGraph{plan: plan, lang: lang, win: win}
}

func (pg *PlanGraph) Widget() fyne.CanvasObject {
	return buildPlanCanvasWidget(pg.plan, pg.lang, 1.0, pg.win)
}

func (pg *PlanGraph) MiniWidget() fyne.CanvasObject {
	return buildMiniPlanWidget(pg.plan, pg.lang)
}

func buildPlanCanvasWidget(plan *parser.QueryPlan, lang *Lang, scale float32, win fyne.Window) fyne.CanvasObject {
	if plan == nil || plan.RootOp == nil {
		return widget.NewLabel("(no plan graph)")
	}

	layoutTree(plan.RootOp)

	pc := newPlanCanvas(plan, lang, scale, win)

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
		pc.fitted = false
		pc.FitToWindow(pc.Size().Width, pc.Size().Height)
		pc.Refresh()
	})

	controls := container.NewHBox(zoomIn, zoomOut, reset)
	return container.NewBorder(controls, nil, nil, nil, pc)
}

func buildMiniPlanWidget(plan *parser.QueryPlan, lang *Lang) fyne.CanvasObject {
	if plan == nil || plan.RootOp == nil {
		return widget.NewLabel("(no plan graph)")
	}

	layoutTree(plan.RootOp)

	pc := newPlanCanvas(plan, lang, 0.55, nil)
	pc.Interactive = false
	pc.FitToWindow(9999, 250)
	return pc
}

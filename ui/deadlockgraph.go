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

const (
	procW = float32(180)
	procH = float32(90)
	resW  = float32(300)
	resH  = float32(100)
)

// DeadlockCanvas draws the visual deadlock graph with draggable nodes.
type DeadlockCanvas struct {
	widget.BaseWidget
	Graph    *parser.DeadlockGraph
	Scale    float32
	OffsetX  float32
	OffsetY  float32
	canvasW  float32
	canvasH  float32
	// drag state
	dragProcIdx int
	dragResIdx  int
}

func newDeadlockCanvas(g *parser.DeadlockGraph) *DeadlockCanvas {
	dc := &DeadlockCanvas{
		Graph:       g,
		Scale:       1.0,
		dragProcIdx: -1,
		dragResIdx:  -1,
	}
	dc.layoutGraph()
	dc.ExtendBaseWidget(dc)
	return dc
}

func (dc *DeadlockCanvas) layoutGraph() {
	if dc.Graph == nil {
		return
	}

	// Typical 2-process deadlock layout from spec
	defaultProcPositions := [][2]float32{
		{80, 200},
		{900, 200},
		{80, 380},
		{900, 380},
	}
	defaultResPositions := [][2]float32{
		{430, 80},
		{430, 320},
		{430, 520},
	}

	for i := range dc.Graph.Processes {
		if i < len(defaultProcPositions) {
			dc.Graph.Processes[i].X = defaultProcPositions[i][0]
			dc.Graph.Processes[i].Y = defaultProcPositions[i][1]
		} else {
			dc.Graph.Processes[i].X = float32(80 + (i%2)*820)
			dc.Graph.Processes[i].Y = float32(200 + (i/2)*200)
		}
	}

	for i := range dc.Graph.Resources {
		if i < len(defaultResPositions) {
			dc.Graph.Resources[i].X = defaultResPositions[i][0]
			dc.Graph.Resources[i].Y = defaultResPositions[i][1]
		} else {
			dc.Graph.Resources[i].X = 430
			dc.Graph.Resources[i].Y = float32(80 + i*220)
		}
	}

	dc.recalcSize()
}

func (dc *DeadlockCanvas) recalcSize() {
	maxX, maxY := float32(1200), float32(500)
	for _, p := range dc.Graph.Processes {
		if p.X+procW > maxX {
			maxX = p.X + procW
		}
		if p.Y+procH > maxY {
			maxY = p.Y + procH
		}
	}
	for _, r := range dc.Graph.Resources {
		if r.X+resW > maxX {
			maxX = r.X + resW
		}
		if r.Y+resH > maxY {
			maxY = r.Y + resH
		}
	}
	dc.canvasW = maxX + 60
	dc.canvasH = maxY + 60
}

func (dc *DeadlockCanvas) CreateRenderer() fyne.WidgetRenderer {
	return &deadlockRenderer{dc: dc}
}

// MouseDown detects which node is under the cursor and sets drag target.
func (dc *DeadlockCanvas) MouseDown(ev *desktop.MouseEvent) {
	dc.dragProcIdx = -1
	dc.dragResIdx = -1

	for i, p := range dc.Graph.Processes {
		nx := p.X*dc.Scale + dc.OffsetX + 10
		ny := p.Y*dc.Scale + dc.OffsetY + 10
		nw := procW * dc.Scale
		nh := procH * dc.Scale
		if ev.Position.X >= nx && ev.Position.X <= nx+nw &&
			ev.Position.Y >= ny && ev.Position.Y <= ny+nh {
			dc.dragProcIdx = i
			return
		}
	}

	for i, r := range dc.Graph.Resources {
		nx := r.X*dc.Scale + dc.OffsetX + 10
		ny := r.Y*dc.Scale + dc.OffsetY + 10
		nw := resW * dc.Scale
		nh := resH * dc.Scale
		if ev.Position.X >= nx && ev.Position.X <= nx+nw &&
			ev.Position.Y >= ny && ev.Position.Y <= ny+nh {
			dc.dragResIdx = i
			return
		}
	}
}

func (dc *DeadlockCanvas) MouseUp(_ *desktop.MouseEvent) {
	dc.dragProcIdx = -1
	dc.dragResIdx = -1
}

func (dc *DeadlockCanvas) Dragged(ev *fyne.DragEvent) {
	if dc.dragProcIdx >= 0 {
		dc.Graph.Processes[dc.dragProcIdx].X += ev.Dragged.DX / dc.Scale
		dc.Graph.Processes[dc.dragProcIdx].Y += ev.Dragged.DY / dc.Scale
		dc.recalcSize()
		dc.Refresh()
		return
	}
	if dc.dragResIdx >= 0 {
		dc.Graph.Resources[dc.dragResIdx].X += ev.Dragged.DX / dc.Scale
		dc.Graph.Resources[dc.dragResIdx].Y += ev.Dragged.DY / dc.Scale
		dc.recalcSize()
		dc.Refresh()
		return
	}
	// Empty space: pan
	dc.OffsetX += ev.Dragged.DX
	dc.OffsetY += ev.Dragged.DY
	dc.Refresh()
}

func (dc *DeadlockCanvas) DragEnd() {}

func (dc *DeadlockCanvas) Scrolled(ev *fyne.ScrollEvent) {
	if ev.Scrolled.DY > 0 {
		dc.Scale *= 1.1
	} else {
		dc.Scale *= 0.9
	}
	if dc.Scale < 0.1 {
		dc.Scale = 0.1
	}
	if dc.Scale > 5.0 {
		dc.Scale = 5.0
	}
	dc.Refresh()
}

func (dc *DeadlockCanvas) MinSize() fyne.Size {
	return fyne.NewSize(dc.canvasW*dc.Scale, dc.canvasH*dc.Scale)
}

// Renderer

type deadlockRenderer struct {
	dc      *DeadlockCanvas
	objects []fyne.CanvasObject
}

func (r *deadlockRenderer) Layout(_ fyne.Size) {}
func (r *deadlockRenderer) MinSize() fyne.Size  { return r.dc.MinSize() }
func (r *deadlockRenderer) Destroy()            {}
func (r *deadlockRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *deadlockRenderer) Refresh() {
	r.objects = nil
	if r.dc.Graph == nil {
		canvas.Refresh(r.dc)
		return
	}

	resIdx := map[string]int{}
	for i, res := range r.dc.Graph.Resources {
		resIdx[res.ID] = i
	}
	procIdx := map[string]int{}
	for i, p := range r.dc.Graph.Processes {
		procIdx[p.ID] = i
	}

	// Draw arrows (behind nodes)
	for _, edge := range r.dc.Graph.Edges {
		pi, pok := procIdx[edge.ProcessID]
		ri, rok := resIdx[edge.ResourceID]
		if !pok || !rok {
			continue
		}
		proc := r.dc.Graph.Processes[pi]
		res := r.dc.Graph.Resources[ri]

		var x1, y1, x2, y2 float32
		var label string
		var col color.Color

		procCX := r.tx(proc.X + procW/2)
		procCY := r.ty(proc.Y + procH/2)
		resCX := r.tx(res.X + resW/2)
		resCY := r.ty(res.Y + resH/2)

		if edge.IsOwner {
			// Process → Resource
			x1, y1 = procCX, procCY
			x2, y2 = resCX, resCY
			label = "Owner: " + edge.Mode
			col = color.RGBA{R: 30, G: 140, B: 30, A: 220}
		} else {
			// Resource → Process
			x1, y1 = resCX, resCY
			x2, y2 = procCX, procCY
			label = "Wait: " + edge.Mode
			col = color.RGBA{R: 180, G: 40, B: 40, A: 220}
		}
		r.drawArrow(x1, y1, x2, y2, label, col)
	}

	// Draw nodes on top
	for i := range r.dc.Graph.Resources {
		r.drawResource(&r.dc.Graph.Resources[i], i == r.dc.dragResIdx)
	}
	for i := range r.dc.Graph.Processes {
		r.drawProcess(&r.dc.Graph.Processes[i], i == r.dc.dragProcIdx)
	}

	canvas.Refresh(r.dc)
}

func (r *deadlockRenderer) tx(x float32) float32 { return x*r.dc.Scale + r.dc.OffsetX + 10 }
func (r *deadlockRenderer) ty(y float32) float32 { return y*r.dc.Scale + r.dc.OffsetY + 10 }

func (r *deadlockRenderer) drawProcess(p *parser.DeadlockProcess, dragging bool) {
	s := r.dc.Scale
	x := r.tx(p.X)
	y := r.ty(p.Y)
	w := procW * s
	h := procH * s

	col := color.RGBA{R: 190, G: 215, B: 255, A: 255}
	if dragging {
		col = color.RGBA{R: 160, G: 190, B: 240, A: 255}
	}
	bg := canvas.NewRectangle(col)
	bg.CornerRadius = 40 * s
	bg.Resize(fyne.NewSize(w, h))
	bg.Move(fyne.NewPos(x, y))

	bord := canvas.NewRectangle(color.Transparent)
	bord.StrokeColor = color.RGBA{R: 70, G: 100, B: 180, A: 200}
	bord.StrokeWidth = 1.5
	bord.CornerRadius = 40 * s
	bord.Resize(fyne.NewSize(w, h))
	bord.Move(fyne.NewPos(x, y))

	r.objects = append(r.objects, bg, bord)

	if p.IsVictim {
		l1 := canvas.NewLine(color.RGBA{R: 200, G: 0, B: 0, A: 255})
		l1.StrokeWidth = 2.5
		l1.Position1 = fyne.NewPos(x+8*s, y+8*s)
		l1.Position2 = fyne.NewPos(x+w-8*s, y+h-8*s)
		l2 := canvas.NewLine(color.RGBA{R: 200, G: 0, B: 0, A: 255})
		l2.StrokeWidth = 2.5
		l2.Position1 = fyne.NewPos(x+w-8*s, y+8*s)
		l2.Position2 = fyne.NewPos(x+8*s, y+h-8*s)
		r.objects = append(r.objects, l1, l2)
	}

	prefix := ""
	if p.IsVictim {
		prefix = "💀 "
	}
	t1 := canvas.NewText(fmt.Sprintf("%sSPID %d", prefix, p.SPID), color.Black)
	t1.TextSize = 10 * s
	t1.TextStyle = fyne.TextStyle{Bold: true}
	t1.Move(fyne.NewPos(x+8*s, y+5*s))

	login := p.Login
	if len(login) > 20 {
		login = login[:20] + "…"
	}
	t2 := canvas.NewText(login, color.RGBA{R: 30, G: 30, B: 30, A: 255})
	t2.TextSize = 9 * s
	t2.Move(fyne.NewPos(x+8*s, y+21*s))

	t3 := canvas.NewText(fmt.Sprintf("Log: %d", p.LogUsed), color.RGBA{R: 60, G: 60, B: 60, A: 255})
	t3.TextSize = 8 * s
	t3.Move(fyne.NewPos(x+8*s, y+38*s))

	wait := p.WaitResource
	if len(wait) > 24 {
		wait = "…" + wait[len(wait)-24:]
	}
	t4 := canvas.NewText(wait, color.RGBA{R: 100, G: 50, B: 0, A: 255})
	t4.TextSize = 8 * s
	t4.Move(fyne.NewPos(x+8*s, y+52*s))

	r.objects = append(r.objects, t1, t2, t3, t4)
}

func (r *deadlockRenderer) drawResource(res *parser.DeadlockResource, dragging bool) {
	s := r.dc.Scale
	x := r.tx(res.X)
	y := r.ty(res.Y)
	w := resW * s
	h := resH * s

	col := color.RGBA{R: 255, G: 255, B: 210, A: 255}
	if dragging {
		col = color.RGBA{R: 240, G: 240, B: 180, A: 255}
	}
	bg := canvas.NewRectangle(col)
	bg.Resize(fyne.NewSize(w, h))
	bg.Move(fyne.NewPos(x, y))

	bord := canvas.NewRectangle(color.Transparent)
	bord.StrokeColor = color.RGBA{R: 140, G: 120, B: 0, A: 220}
	bord.StrokeWidth = 1.5
	bord.Resize(fyne.NewSize(w, h))
	bord.Move(fyne.NewPos(x, y))

	r.objects = append(r.objects, bg, bord)

	t1 := canvas.NewText(res.LockType, color.Black)
	t1.TextSize = 10 * s
	t1.TextStyle = fyne.TextStyle{Bold: true}
	t1.Move(fyne.NewPos(x+6*s, y+5*s))

	obj := shortObjName(res.ObjectName, 34)
	t2 := canvas.NewText(obj, color.RGBA{R: 40, G: 40, B: 40, A: 255})
	t2.TextSize = 8 * s
	t2.Move(fyne.NewPos(x+6*s, y+21*s))

	if res.IndexName != "" {
		idx := shortObjName(res.IndexName, 34)
		t3 := canvas.NewText("Idx: "+idx, color.RGBA{R: 80, G: 60, B: 0, A: 255})
		t3.TextSize = 8 * s
		t3.Move(fyne.NewPos(x+6*s, y+37*s))
		r.objects = append(r.objects, t3)
	}

	r.objects = append(r.objects, t1, t2)
}

func (r *deadlockRenderer) drawArrow(x1, y1, x2, y2 float32, label string, col color.Color) {
	line := canvas.NewLine(col)
	line.StrokeWidth = 1.5
	line.Position1 = fyne.NewPos(x1, y1)
	line.Position2 = fyne.NewPos(x2, y2)
	r.objects = append(r.objects, line)

	dx := x2 - x1
	dy := y2 - y1
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if length < 1 {
		return
	}
	ux, uy := dx/length, dy/length
	arrowLen := float32(12)
	arrowW := float32(5)

	a1 := canvas.NewLine(col)
	a1.StrokeWidth = 1.5
	a1.Position1 = fyne.NewPos(x2, y2)
	a1.Position2 = fyne.NewPos(x2-arrowLen*ux+arrowW*uy, y2-arrowLen*uy-arrowW*ux)
	a2 := canvas.NewLine(col)
	a2.StrokeWidth = 1.5
	a2.Position1 = fyne.NewPos(x2, y2)
	a2.Position2 = fyne.NewPos(x2-arrowLen*ux-arrowW*uy, y2-arrowLen*uy+arrowW*ux)
	r.objects = append(r.objects, a1, a2)

	lbl := canvas.NewText(label, col)
	lbl.TextSize = 9
	lbl.Move(fyne.NewPos((x1+x2)/2+4, (y1+y2)/2-14))
	r.objects = append(r.objects, lbl)
}

func shortObjName(s string, max int) string {
	parts := strings.Split(s, ".")
	last := parts[len(parts)-1]
	if len(last) > max {
		return "…" + last[len(last)-max:]
	}
	return last
}

// NewDeadlockGraph builds the full deadlock view: info panel + visual canvas.
func NewDeadlockGraph(g *parser.DeadlockGraph, lang *Lang) fyne.CanvasObject {
	if g == nil {
		return widget.NewLabel("(no deadlock data)")
	}

	dc := newDeadlockCanvas(g)

	zoomIn := widget.NewButton("+", func() {
		dc.Scale *= 1.2
		if dc.Scale > 5.0 {
			dc.Scale = 5.0
		}
		dc.Refresh()
	})
	zoomOut := widget.NewButton("-", func() {
		dc.Scale *= 0.8
		if dc.Scale < 0.1 {
			dc.Scale = 0.1
		}
		dc.Refresh()
	})
	reset := widget.NewButton("Reset", func() {
		dc.Scale = 1.0
		dc.OffsetX = 0
		dc.OffsetY = 0
		dc.layoutGraph()
		dc.Refresh()
	})

	hint := widget.NewLabel("Drag nodes to reposition  |  Scroll to zoom  |  Drag empty space to pan")
	hint.TextStyle = fyne.TextStyle{Italic: true}

	controls := container.NewHBox(zoomIn, zoomOut, reset, hint)
	scroll := container.NewScroll(dc)
	scroll.SetMinSize(fyne.NewSize(400, 300))

	return container.NewBorder(controls, nil, nil, nil, scroll)
}

// BuildDeadlockInfoPanel builds the structured text summary panel.
func BuildDeadlockInfoPanel(g *parser.DeadlockGraph) fyne.CanvasObject {
	rows := []fyne.CanvasObject{}

	title := widget.NewLabel(fmt.Sprintf("🔴 Deadlock  |  %d protsessi  |  %d ressurssi",
		len(g.Processes), len(g.Resources)))
	title.TextStyle = fyne.TextStyle{Bold: true}
	rows = append(rows, title)

	for _, proc := range g.Processes {
		victimMark := ""
		if proc.IsVictim {
			victimMark = " ⚠️ VICTIM"
		}
		sql := strings.TrimSpace(proc.InputBuf)
		if len(sql) > 120 {
			sql = sql[:120] + "..."
		}
		iso := proc.IsolationLevel
		if len(iso) > 30 {
			iso = iso[:30]
		}
		text := fmt.Sprintf("SPID %d%s  |  %s  |  Log: %d\nWait: %s  |  %s\n%s",
			proc.SPID, victimMark, proc.Login, proc.LogUsed,
			proc.WaitResource, iso, sql)
		lbl := widget.NewLabel(text)
		lbl.Wrapping = fyne.TextWrapWord
		rows = append(rows, lbl)
	}

	for i, res := range g.Resources {
		ownerSPID := parser.FindProcessSPID(g, res.OwnerProcessID)
		waiterSPID := parser.FindProcessSPID(g, res.WaiterProcessID)
		text := fmt.Sprintf("Lock %d: %s  |  %s\nIndex: %s\nOwner: SPID %d (%s)  →  Waiter: SPID %d (%s)",
			i+1, res.LockType, shortObjName(res.ObjectName, 50),
			shortObjName(res.IndexName, 50),
			ownerSPID, res.OwnerMode, waiterSPID, res.WaiterMode)
		lbl := widget.NewLabel(text)
		lbl.Wrapping = fyne.TextWrapWord
		rows = append(rows, lbl)
	}

	return container.NewScroll(container.NewVBox(rows...))
}

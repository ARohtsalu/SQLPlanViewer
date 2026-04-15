package ui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"sqlplanviewer/parser"
)

const (
	procW  = float32(180)
	procH  = float32(90)
	resW   = float32(300)
	resH   = float32(100)
)

// DeadlockCanvas draws the visual deadlock graph.
type DeadlockCanvas struct {
	widget.BaseWidget
	Graph   *parser.DeadlockGraph
	Scale   float32
	OffsetX float32
	OffsetY float32
	canvasW float32
	canvasH float32
}

func newDeadlockCanvas(g *parser.DeadlockGraph) *DeadlockCanvas {
	dc := &DeadlockCanvas{Graph: g, Scale: 1.0}
	dc.layoutGraph()
	dc.ExtendBaseWidget(dc)
	return dc
}

// layoutGraph assigns positions to all processes and resources.
func (dc *DeadlockCanvas) layoutGraph() {
	if dc.Graph == nil {
		return
	}
	procs := dc.Graph.Processes
	res := dc.Graph.Resources

	// Resources: center column
	centerX := float32(300)
	resStartY := float32(60)
	for i := range res {
		res[i].X = centerX
		res[i].Y = resStartY + float32(i)*float32(resH+60)
	}

	// Processes: split left / right of resources
	leftX := float32(20)
	rightX := centerX + resW + 60

	for i := range procs {
		if i%2 == 0 {
			procs[i].X = leftX
		} else {
			procs[i].X = rightX
		}
		procs[i].Y = float32(30) + float32(i/2)*float32(procH+80)
	}

	// Calculate total canvas size
	maxX, maxY := float32(0), float32(0)
	for _, p := range procs {
		if p.X+procW > maxX {
			maxX = p.X + procW
		}
		if p.Y+procH > maxY {
			maxY = p.Y + procH
		}
	}
	for _, r := range res {
		if r.X+resW > maxX {
			maxX = r.X + resW
		}
		if r.Y+resH > maxY {
			maxY = r.Y + resH
		}
	}
	dc.canvasW = maxX + 40
	dc.canvasH = maxY + 40
}

func (dc *DeadlockCanvas) CreateRenderer() fyne.WidgetRenderer {
	return &deadlockRenderer{dc: dc}
}

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

func (dc *DeadlockCanvas) Dragged(ev *fyne.DragEvent) {
	dc.OffsetX += ev.Dragged.DX
	dc.OffsetY += ev.Dragged.DY
	dc.Refresh()
}

func (dc *DeadlockCanvas) DragEnd() {}

func (dc *DeadlockCanvas) MinSize() fyne.Size {
	return fyne.NewSize(dc.canvasW*dc.Scale, dc.canvasH*dc.Scale)
}

type deadlockRenderer struct {
	dc      *DeadlockCanvas
	objects []fyne.CanvasObject
}

func (r *deadlockRenderer) Layout(_ fyne.Size) {}
func (r *deadlockRenderer) MinSize() fyne.Size { return r.dc.MinSize() }
func (r *deadlockRenderer) Destroy()           {}
func (r *deadlockRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *deadlockRenderer) Refresh() {
	r.objects = nil
	if r.dc.Graph == nil {
		canvas.Refresh(r.dc)
		return
	}

	// Build resource lookup by ID
	resIdx := map[string]int{}
	for i, res := range r.dc.Graph.Resources {
		resIdx[res.ID] = i
	}
	procIdx := map[string]int{}
	for i, p := range r.dc.Graph.Processes {
		procIdx[p.ID] = i
	}

	// Draw edges first (behind nodes)
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

		if edge.IsOwner {
			// Arrow: process → resource
			x1 = r.tx(proc.X + procW)
			y1 = r.ty(proc.Y + procH/2)
			x2 = r.tx(res.X)
			y2 = r.ty(res.Y + resH/2)
			label = "Owner: " + edge.Mode
			col = color.RGBA{R: 50, G: 130, B: 50, A: 220}
		} else {
			// Arrow: resource → process (waiting)
			x1 = r.tx(res.X + resW)
			y1 = r.ty(res.Y + resH/2)
			x2 = r.tx(proc.X + procW)
			y2 = r.ty(proc.Y + procH/2)
			if proc.X < res.X {
				// process is on the left
				x1 = r.tx(res.X)
				x2 = r.tx(proc.X + procW)
			}
			label = "Wait: " + edge.Mode
			col = color.RGBA{R: 180, G: 50, B: 50, A: 220}
		}
		r.drawArrow(x1, y1, x2, y2, label, col)
	}

	// Draw resource rectangles
	for _, res := range r.dc.Graph.Resources {
		r.drawResource(&res)
	}

	// Draw process ellipses
	for _, proc := range r.dc.Graph.Processes {
		r.drawProcess(&proc)
	}

	canvas.Refresh(r.dc)
}

func (r *deadlockRenderer) tx(x float32) float32 { return x*r.dc.Scale + r.dc.OffsetX + 10 }
func (r *deadlockRenderer) ty(y float32) float32 { return y*r.dc.Scale + r.dc.OffsetY + 10 }

func (r *deadlockRenderer) drawProcess(p *parser.DeadlockProcess) {
	s := r.dc.Scale
	x := r.tx(p.X)
	y := r.ty(p.Y)
	w := procW * s
	h := procH * s

	col := color.RGBA{R: 190, G: 215, B: 255, A: 255}
	bg := canvas.NewRectangle(col)
	bg.CornerRadius = 40 * s
	bg.Resize(fyne.NewSize(w, h))
	bg.Move(fyne.NewPos(x, y))

	bord := canvas.NewRectangle(color.Transparent)
	bord.StrokeColor = color.RGBA{R: 80, G: 80, B: 180, A: 200}
	bord.StrokeWidth = 1.5
	bord.CornerRadius = 40 * s
	bord.Resize(fyne.NewSize(w, h))
	bord.Move(fyne.NewPos(x, y))

	r.objects = append(r.objects, bg, bord)

	// X mark for victim
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

	spidLabel := fmt.Sprintf("SPID: %d", p.SPID)
	if p.IsVictim {
		spidLabel = "💀 " + spidLabel
	}
	t1 := canvas.NewText(spidLabel, color.Black)
	t1.TextSize = 10 * s
	t1.TextStyle = fyne.TextStyle{Bold: true}
	t1.Move(fyne.NewPos(x+8*s, y+6*s))

	login := p.Login
	if len(login) > 20 {
		login = login[:20] + "…"
	}
	t2 := canvas.NewText(login, color.RGBA{R: 40, G: 40, B: 40, A: 255})
	t2.TextSize = 9 * s
	t2.Move(fyne.NewPos(x+8*s, y+22*s))

	wait := p.WaitResource
	if len(wait) > 22 {
		wait = wait[:22] + "…"
	}
	t3 := canvas.NewText("Wait: "+wait, color.RGBA{R: 80, G: 40, B: 0, A: 255})
	t3.TextSize = 8 * s
	t3.Move(fyne.NewPos(x+8*s, y+38*s))

	logStr := fmt.Sprintf("Log: %d", p.LogUsed)
	t4 := canvas.NewText(logStr, color.RGBA{R: 60, G: 60, B: 60, A: 255})
	t4.TextSize = 8 * s
	t4.Move(fyne.NewPos(x+8*s, y+52*s))

	r.objects = append(r.objects, t1, t2, t3, t4)
}

func (r *deadlockRenderer) drawResource(res *parser.DeadlockResource) {
	s := r.dc.Scale
	x := r.tx(res.X)
	y := r.ty(res.Y)
	w := resW * s
	h := resH * s

	bg := canvas.NewRectangle(color.RGBA{R: 255, G: 255, B: 220, A: 255})
	bg.Resize(fyne.NewSize(w, h))
	bg.Move(fyne.NewPos(x, y))

	bord := canvas.NewRectangle(color.Transparent)
	bord.StrokeColor = color.RGBA{R: 140, G: 120, B: 0, A: 220}
	bord.StrokeWidth = 1.5
	bord.Resize(fyne.NewSize(w, h))
	bord.Move(fyne.NewPos(x, y))

	r.objects = append(r.objects, bg, bord)

	t1 := canvas.NewText(res.Type+" ("+res.Mode+")", color.Black)
	t1.TextSize = 10 * s
	t1.TextStyle = fyne.TextStyle{Bold: true}
	t1.Move(fyne.NewPos(x+6*s, y+5*s))

	objName := shortName(res.ObjectName, 35)
	t2 := canvas.NewText(objName, color.RGBA{R: 40, G: 40, B: 40, A: 255})
	t2.TextSize = 8 * s
	t2.Move(fyne.NewPos(x+6*s, y+22*s))

	if res.IndexName != "" {
		idxName := shortName(res.IndexName, 35)
		t3 := canvas.NewText("Idx: "+idxName, color.RGBA{R: 80, G: 60, B: 0, A: 255})
		t3.TextSize = 8 * s
		t3.Move(fyne.NewPos(x+6*s, y+38*s))
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

	// Arrowhead
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

	// Label at midpoint
	lbl := canvas.NewText(label, col)
	lbl.TextSize = 9
	lbl.Move(fyne.NewPos((x1+x2)/2+4, (y1+y2)/2-14))
	r.objects = append(r.objects, lbl)
}

func shortName(s string, max int) string {
	// Show last N chars (table name is more useful than DB prefix)
	parts := strings.Split(s, ".")
	last := parts[len(parts)-1]
	if len(last) > max {
		return "…" + last[len(last)-max:]
	}
	return last
}

// NewDeadlockGraph is the public entry point.
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
		dc.Refresh()
	})

	controls := container.NewHBox(zoomIn, zoomOut, reset)
	scroll := container.NewScroll(dc)
	scroll.SetMinSize(fyne.NewSize(400, 300))

	return container.NewBorder(controls, nil, nil, nil, scroll)
}

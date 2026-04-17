# SQLPlanViewer

A lightweight, fast SQL Server execution plan viewer for Windows — single exe, no installation required.

Designed for quickly browsing large collections of plan files, similar to how IrfanView flips through images.

**[Download latest release](https://github.com/ARohtsalu/SQLPlanViewer/releases/latest/download/sqlplanviewer.exe)**

---

## Features

### File Management
- Open a folder (`.sqlplan` and `.xdl` files)
- Arrow keys ↑↓ navigate between files
- Last opened folder is restored on startup
- Copy file path to clipboard

### Execution Plans (`.sqlplan`)
- All statements shown as tabs — most expensive query selected automatically
- Tabs color-coded by cost: gray (0%), warm yellow (0–10%), amber (10–25%), matte red (≥25%)
- Visual tree: SSMS-style left-to-right layout, L-shaped connectors, log-scaled edge thickness
- 113 SSMS-style PNG icons for operator types
- Hover tooltip: cost %, I/O and CPU, object, predicate, output columns, warnings
- Click a node to pin the tooltip; click again or click empty space to dismiss
- Zoom: scroll wheel, +/− buttons, auto-fit to window
- Auto-refit when window is resized significantly (e.g. moved to another monitor)

### Deadlock Graphs (`.xdl`)
- Visual deadlock graph with draggable nodes
- Processes: SPID, login, isolation level, SQL text
- Resources: waiting vs. owning locks

### Warnings
- Missing indexes (per table with impact %)
- Table scans
- No statistics, TempDB spill, no join predicate

### Toolbar
- Open in SSMS: auto-detects SSMS path, prompts if not found
- Open in Performance Studio (Erik Darling's tool): supports Browse button for path selection
- Language toggle: ET | EN

---

## Requirements

- Windows 10/11 (64-bit)
- SQL Server Management Studio (optional, for "Open in SSMS")
- [Performance Studio](https://github.com/erikdarlingdata/PerformanceStudio) (optional)

---

## Building

Requires: Go 1.22+, GCC (MSYS2 ucrt64 or mingw64), Fyne v2.

```bash
# In MSYS2 ucrt64 shell:
pacman -S mingw-w64-ucrt-x86_64-gcc

# In the project root:
go build -ldflags="-H windowsgui" -o sqlplanviewer.exe .
```

Produces a single `sqlplanviewer.exe` with no external dependencies.

---

## Settings

`settings.json` (stored in `%APPDATA%\sqlplanviewer\`):

| Field | Description |
|-------|-------------|
| `ssmsPath` | Path to SSMS executable |
| `performanceStudioPath` | Path to PlanViewer.App.exe |
| `lastFolder` | Last opened folder |
| `language` | `EN` or `ET` |

---

## About

Built with Go + [Fyne v2](https://fyne.io/).

Special thanks to [Erik Darling](https://github.com/erikdarlingdata) for [Performance Studio](https://github.com/erikdarlingdata/PerformanceStudio) — the visual design, node layout, and operator icon set are directly inspired by his work. If you need a full-featured plan viewer, use his tool.

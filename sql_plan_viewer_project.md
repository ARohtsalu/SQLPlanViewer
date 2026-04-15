# SQL Plan Viewer — Project Plan for Claude Code

## Overview
Build a standalone Windows desktop application (`SqlPlanViewer.exe`) in Go that allows a DBA to browse a folder of `.sqlplan` and `.xdl` files, view a visual query plan summary in the right pane, and open files in SSMS when needed. No installer required — single binary.

---

## Tech Stack
- **Language:** Go (1.21+)
- **GUI framework:** [Fyne](https://fyne.io/) v2 (`fyne.io/fyne/v2`)
- **XML parsing:** Go standard library `encoding/xml`
- **Build output:** `SqlPlanViewer.exe` (Windows, no console window)

---

## UI Layout

```
┌─────────────────────────────────────────────────────────────────────┐
│  [📁 Open Folder]   SQL Plan Viewer          [ET | EN]              │
├───────────────────────┬─────────────────────────────────────────────┤
│  File Tree (left)     │  Plan View (right)                          │
│                       │                                             │
│  📄 query1.sqlplan    │  ┌──────────────────────────────────────┐   │
│  📄 query2.sqlplan    │  │  Statement: SELECT ...               │   │
│  ⚠️  slow.sqlplan     │  │  Est. Cost: 1.234   Rows: 50 000     │   │
│  🔴 deadlock1.xdl    │  │                                      │   │
│  📄 query3.sqlplan    │  │  [Plan Graph - nodes + arrows]       │   │
│                       │  │                                      │   │
│                       │  │  ⚠️  Warnings found: 2              │   │
│                       │  │  🔴 Table Scans: 1                   │   │
│                       │  │  🟡 Index Scans: 3                   │   │
│                       │  └──────────────────────────────────────┘   │
│                       │                                             │
│                       │  [Open in SSMS]  [Copy Path]               │
└───────────────────────┴─────────────────────────────────────────────┘
```

---

## Features

### 1. File Tree (Left Pane)
- Button to select root folder (recursive scan)
- Show only `.sqlplan` and `.xdl` files
- File icons:
  - 🔴 `.xdl` = deadlock graph
  - ⚠️  `.sqlplan` with warnings or scans
  - 📄 `.sqlplan` normal
- Click on file → load in right pane
- File list is scrollable

### 2. Plan View (Right Pane) — `.sqlplan` files

**Header info:**
- SQL statement text (first 200 chars)
- Estimated subtree cost (root node)
- Estimated rows

**Visual Plan Graph:**
- Parse `ShowPlanXML` → `RelOp` tree recursively
- Draw nodes as rounded boxes with:
  - Operator name (e.g. `Hash Match`, `Table Scan`, `Index Seek`)
  - Cost % of total (from `EstimatedTotalSubtreeCost`)
  - Estimated rows
- Connect nodes with arrows (parent → children)
- Color coding:
  - 🔴 Red: `Table Scan`, `Index Scan` (full scans)
  - 🟠 Orange: `Sort`, `Spool`, `Hash Match` (expensive)
  - 🟡 Yellow: cost > 30% of total
  - 🟢 Green: `Index Seek`, `Nested Loops` (efficient)
- Nodes are scrollable/zoomable if plan is large

**Alerts bar (below graph):**
- Warnings count (from `<Warnings>` elements)
- Table Scan count
- Index Scan count  
- Spool count
- Missing Index hints (from `<MissingIndexes>`)

### 3. Plan View (Right Pane) — `.xdl` files (Deadlock Graph)

**Parse deadlock XML:**
- Show victim process (highlighted in red)
- List all processes involved: SPID, login, wait resource, query snippet
- List all resources (locks): type, object name
- Simple visual: process nodes ↔ resource nodes with arrows
- Indicate deadlock victim clearly

### 4. Language Toggle (ET/EN)
All UI labels, buttons, column headers switchable between Estonian and English.

**Estonian strings:**
- "Ava kaust" / "Open Folder"
- "Ava SSMS-is" / "Open in SSMS"
- "Kopeeri tee" / "Copy Path"
- "Hoiatused" / "Warnings"
- "Puuduvad indeksid" / "Missing Indexes"
- "Täielik skannimine" / "Table Scan"
- "Operaator" / "Operator"
- "Kulu %" / "Cost %"
- "Hinnangulised read" / "Est. Rows"

### 5. Open in SSMS
- Button: opens the selected file in SSMS using shell execute
- SSMS path: try common install paths:
  - `C:\Program Files (x86)\Microsoft SQL Server Management Studio 20\Common7\IDE\Ssms.exe`
  - `C:\Program Files (x86)\Microsoft SQL Server Management Studio 19\Common7\IDE\Ssms.exe`
  - `C:\Program Files\Microsoft SQL Server Management Studio 20\Common7\IDE\Ssms.exe`
- Command: `ssms.exe "<filepath>"`
- If SSMS not found: show dialog with path input to configure manually

### 6. Copy Path
- Copies full file path to clipboard

---

## Project Structure

```
SqlPlanViewer/
├── main.go                  # Entry point, window setup, layout
├── ui/
│   ├── filetree.go          # Left pane: folder browser, file list
│   ├── planview.go          # Right pane: plan display coordinator
│   ├── plangraph.go         # Canvas drawing: nodes, arrows, colors
│   ├── deadlockview.go      # XDL deadlock display
│   └── lang.go              # ET/EN language strings
├── parser/
│   ├── sqlplan.go           # Parse .sqlplan XML → Go structs
│   └── deadlock.go          # Parse .xdl XML → Go structs
├── go.mod
└── go.sum
```

---

## Key Go Structs

### sqlplan.go
```go
type QueryPlan struct {
    StatementText     string
    TotalCost         float64
    EstimatedRows     float64
    MissingIndexes    []MissingIndex
    Warnings          []Warning
    RootOp            *RelOp
}

type RelOp struct {
    NodeID            int
    PhysicalOp        string   // "Table Scan", "Index Seek", etc.
    LogicalOp         string
    EstimatedCost     float64
    EstimatedRows     float64
    CostPercent       float64  // calculated: EstimatedCost / TotalCost * 100
    Children          []*RelOp
}

type MissingIndex struct {
    Database  string
    Table     string
    Columns   string
    Impact    float64
}
```

### deadlock.go
```go
type DeadlockGraph struct {
    Victim    string
    Processes []DeadlockProcess
    Resources []DeadlockResource
}

type DeadlockProcess struct {
    ID           string
    SPID         int
    Login        string
    WaitResource string
    IsVictim     bool
    QueryText    string
}
```

---

## Build Instructions (for Claude Code to execute)

```bash
# Install dependencies
go mod init sqlplanviewer
go get fyne.io/fyne/v2@latest
go get fyne.io/fyne/v2/cmd/fyne

# Build Windows exe (no console window)
GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui" -o SqlPlanViewer.exe .

# Or on Windows directly:
go build -ldflags="-H windowsgui" -o SqlPlanViewer.exe .
```

---

## XML Parsing Notes

### .sqlplan format (ShowPlanXML)
Root element: `<ShowPlanXML>`  
Statement node: `//Batch/Statements/StmtSimple`  
- Attribute `StatementText` = SQL text  
- Attribute `StatementSubTreeCost` = total cost  
- Child `<QueryPlan>` → child `<RelOp>` = root operator  

RelOp attributes: `NodeId`, `PhysicalOp`, `LogicalOp`, `EstimatedTotalSubtreeCost`, `EstimatedRows`  
Children: `<RelOp>` nested inside operator-specific elements (e.g. `<Hash><RelOp>`, `<NestedLoops><RelOp>`)  
Warnings: `<QueryPlan><Warnings>` → `<ColumnsWithNoStatistics>`, `<SpillToTempDb>`, etc.  
Missing indexes: `<QueryPlan><MissingIndexes><MissingIndexGroup>`

### .xdl format (deadlock graph)
Root element: `<deadlock>`  
Victim: attribute `victim` on `<deadlock>` = process ID  
Processes: `<process-list><process>` — attributes: `id`, `spid`, `loginname`, `waitresource`  
Query text: `<process><executionStack><frame>` or `<inputbuf>`  
Resources: `<resource-list>` → various lock types

---

## Priority Order for Claude Code
1. Project scaffold + `go.mod`
2. XML parsers (`parser/sqlplan.go`, `parser/deadlock.go`)
3. File tree UI (`ui/filetree.go`)
4. Plan graph drawing (`ui/plangraph.go`) — most complex
5. Deadlock view (`ui/deadlockview.go`)
6. Language toggle (`ui/lang.go`)
7. SSMS integration + clipboard
8. Build + test with sample files

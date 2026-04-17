package parser

import (
	"bytes"
	"encoding/xml"
	"os"
	"strconv"
	"strings"
)

type QueryPlan struct {
	StatementIndex int
	StatementText  string
	TotalCost      float64
	EstimatedRows  float64
	MissingIndexes []MissingIndex
	Warnings       []Warning
	RootOp         *RelOp
}

type RelOp struct {
	NodeID        int
	PhysicalOp    string
	LogicalOp     string
	EstimatedCost float64
	EstimatedRows float64
	CostPercent   float64
	X, Y          float32
	CachedHeight  float32 // set by UI layout, avoids repeated calculation
	Children      []*RelOp

	// Extended tooltip fields
	EstRowsRead      float64
	EstimateIO       float64
	EstimateCPU      float64
	AvgRowSize       float64
	TableCardinality float64
	Parallel         bool
	EstRebinds       float64
	EstRewinds       float64
	ExecutionMode    string
	ObjDatabase      string
	ObjSchema        string
	ObjTable         string
	ObjIndex         string
	ObjIndexKind     string
	ObjStorage       string
	OutputColumns    int
	OutputColNames   []string // output column reference names
	Predicate        string   // residual predicate expression (ScalarString)
	SeekPredicate    string   // seek predicate expression (ScalarString)
	HasWarnings      bool
	WarningTexts     []string
}

type MissingIndex struct {
	Database string
	Table    string
	Columns  string
	Impact   float64
}

type Warning struct {
	Text string
}

// XML structs

type xmlShowPlanXML struct {
	XMLName xml.Name   `xml:"ShowPlanXML"`
	Batches []xmlBatch `xml:"BatchSequence>Batch"`
}

type xmlBatch struct {
	Statements []xmlStmt `xml:"Statements>StmtSimple"`
}

type xmlStmt struct {
	StatementText string    `xml:"StatementText,attr"`
	SubTreeCost   float64   `xml:"StatementSubTreeCost,attr"`
	EstimatedRows float64   `xml:"StatementEstRows,attr"`
	QueryPlan     *xmlQPlan `xml:"QueryPlan"`
}

type xmlQPlan struct {
	MissingIndexes []xmlMissingIndexGroup `xml:"MissingIndexes>MissingIndexGroup"`
	Warnings       *xmlWarnings           `xml:"Warnings"`
	RelOp          *xmlRelOp              `xml:"RelOp"`
}

type xmlMissingIndexGroup struct {
	Impact  float64         `xml:"Impact,attr"`
	Indexes []xmlMissingIdx `xml:"MissingIndex"`
}

type xmlMissingIdx struct {
	Database     string        `xml:"Database,attr"`
	Table        string        `xml:"Table,attr"`
	ColumnGroups []xmlColGroup `xml:"ColumnGroup"`
}

type xmlColGroup struct {
	Columns []xmlCol `xml:"Column"`
}

type xmlCol struct {
	Name string `xml:"Name,attr"`
}

type xmlWarnings struct {
	ColumnsNoStats []xmlColNoStats `xml:"ColumnsWithNoStatistics>ColumnReference"`
	SpillToTempDb  []struct{}      `xml:"SpillToTempDb"`
}

type xmlColNoStats struct {
	Table  string `xml:"Table,attr"`
	Column string `xml:"Column,attr"`
}

type xmlRelOp struct {
	NodeID      int     `xml:"NodeId,attr"`
	PhysicalOp  string  `xml:"PhysicalOp,attr"`
	LogicalOp   string  `xml:"LogicalOp,attr"`
	SubtreeCost float64 `xml:"EstimatedTotalSubtreeCost,attr"`
	EstRows     float64 `xml:"EstimateRows,attr"`
	EstRowsRead float64 `xml:"EstimatedRowsRead,attr"`
	EstimateIO  float64 `xml:"EstimateIO,attr"`
	EstimateCPU float64 `xml:"EstimateCPU,attr"`
	AvgRowSize  float64 `xml:"AvgRowSize,attr"`
	TableCard   float64 `xml:"TableCardinality,attr"`
	ParallelStr string  `xml:"Parallel,attr"`
	EstRebinds  float64 `xml:"EstimateRebinds,attr"`
	EstRewinds  float64 `xml:"EstimateRewinds,attr"`
	ExecMode    string  `xml:"EstimatedExecutionMode,attr"`
	InnerXML    []byte  `xml:",innerxml"`
}

// xmlObject represents <Object ...> child element inside a RelOp
type xmlObject struct {
	Database  string `xml:"Database,attr"`
	Schema    string `xml:"Schema,attr"`
	Table     string `xml:"Table,attr"`
	Index     string `xml:"Index,attr"`
	IndexKind string `xml:"IndexKind,attr"`
	Storage   string `xml:"Storage,attr"`
}

// xmlWarningNode represents <Warnings> inside a RelOp
type xmlWarningNode struct {
	ColumnsNoStats []struct {
		Table  string `xml:"Table,attr"`
		Column string `xml:"Column,attr"`
	} `xml:"ColumnsWithNoStatistics>ColumnReference"`
	SpillToTempDb []struct {
		SpillLevel int    `xml:"SpillLevel,attr"`
		SpillType  string `xml:"SpillType,attr"`
	} `xml:"SpillToTempDb"`
	NoJoinPredicate []struct{} `xml:"NoJoinPredicate"`
}

// xmlOutputList wraps <OutputList><ColumnReference .../> ...
type xmlOutputList struct {
	Columns []struct{} `xml:"ColumnReference"`
}

// ParseSqlPlan parses all statements from a .sqlplan file.
// Returns all statements that have a QueryPlan (skips DDL/SET with no plan).
func ParseSqlPlan(path string) ([]*QueryPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Strip XML namespace so struct tags match without namespace prefix
	data = bytes.ReplaceAll(data,
		[]byte(`xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"`),
		[]byte(""))

	var root xmlShowPlanXML
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	var plans []*QueryPlan
	idx := 1

	for _, batch := range root.Batches {
		for _, stmt := range batch.Statements {
			// Skip DDL/SET/DECLARE wrappers that have no QueryPlan element —
			// they carry no execution plan and only add noise to the tab list.
			if stmt.QueryPlan == nil {
				continue
			}

			qp := &QueryPlan{
				StatementIndex: idx,
				StatementText:  stmt.StatementText,
				TotalCost:      stmt.SubTreeCost,
				EstimatedRows:  stmt.EstimatedRows,
			}
			idx++

			if stmt.QueryPlan != nil {
				for _, mig := range stmt.QueryPlan.MissingIndexes {
					for _, mi := range mig.Indexes {
						cols := ""
						for _, cg := range mi.ColumnGroups {
							for i, c := range cg.Columns {
								if i > 0 {
									cols += ", "
								}
								cols += c.Name
							}
						}
						qp.MissingIndexes = append(qp.MissingIndexes, MissingIndex{
							Database: mi.Database,
							Table:    mi.Table,
							Columns:  cols,
							Impact:   mig.Impact,
						})
					}
				}

				if w := stmt.QueryPlan.Warnings; w != nil {
					for _, c := range w.ColumnsNoStats {
						qp.Warnings = append(qp.Warnings, Warning{
							Text: "No statistics: " + c.Table + "." + c.Column,
						})
					}
					for range w.SpillToTempDb {
						qp.Warnings = append(qp.Warnings, Warning{Text: "Spill to TempDB"})
					}
				}

				if stmt.QueryPlan.RelOp != nil {
					qp.RootOp = convertRelOp(stmt.QueryPlan.RelOp, qp.TotalCost)
				}
			}

			plans = append(plans, qp)
		}
	}

	return plans, nil
}

// MostExpensiveIndex returns the index of the statement with the highest cost.
func MostExpensiveIndex(plans []*QueryPlan) int {
	best, bestCost := 0, -1.0
	for i, p := range plans {
		if p.TotalCost > bestCost {
			bestCost = p.TotalCost
			best = i
		}
	}
	return best
}

// BatchTotal returns the sum of all statement costs.
func BatchTotal(plans []*QueryPlan) float64 {
	total := 0.0
	for _, p := range plans {
		total += p.TotalCost
	}
	return total
}

// HasWarningsOrScans returns true if any statement in the slice has warnings or table scans.
func HasWarningsOrScans(plans []*QueryPlan) bool {
	for _, p := range plans {
		if len(p.Warnings) > 0 || len(p.MissingIndexes) > 0 {
			return true
		}
		if countOp(p.RootOp, "Table Scan") > 0 {
			return true
		}
	}
	return false
}

func countOp(op *RelOp, name string) int {
	if op == nil {
		return 0
	}
	n := 0
	if op.PhysicalOp == name {
		n = 1
	}
	for _, c := range op.Children {
		n += countOp(c, name)
	}
	return n
}

// extractFirstScalarString walks the tokens inside an already-started XML element
// and returns the ScalarString attribute of the first ScalarOperator found.
// Consumes all tokens up to and including the matching end element.
func extractFirstScalarString(dec *xml.Decoder, _ xml.StartElement) string {
	var result string
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			depth++
			if tok.Name.Local == "ScalarOperator" && result == "" {
				for _, a := range tok.Attr {
					if a.Name.Local == "ScalarString" {
						result = a.Value
					}
				}
			}
		case xml.EndElement:
			depth--
		}
	}
	return result
}

// parseRelOpInnerXML does a single pass over the InnerXML of a RelOp:
// - Extracts Object, OutputList, and Warnings info into op
// - Collects and returns direct-child RelOp elements
// DecodeElement on child <RelOp> elements consumes their full subtrees,
// so nested Object/Warnings are not double-counted.
func parseRelOpInnerXML(op *RelOp, innerXML []byte) []*xmlRelOp {
	dec := xml.NewDecoder(bytes.NewReader(innerXML))
	var children []*xmlRelOp

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		t, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch t.Name.Local {
		case "RelOp":
			var ro xmlRelOp
			if err := dec.DecodeElement(&ro, &t); err == nil {
				children = append(children, &ro)
			}
		case "Object":
			for _, attr := range t.Attr {
				switch attr.Name.Local {
				case "Database":
					op.ObjDatabase = strings.Trim(attr.Value, "[]")
				case "Schema":
					op.ObjSchema = strings.Trim(attr.Value, "[]")
				case "Table":
					op.ObjTable = strings.Trim(attr.Value, "[]")
				case "Index":
					op.ObjIndex = strings.Trim(attr.Value, "[]")
				case "IndexKind":
					op.ObjIndexKind = attr.Value
				case "Storage":
					op.ObjStorage = attr.Value
				}
			}
		case "OutputList":
			var cols []string
			depth := 1
			for depth > 0 {
				inner, ierr := dec.Token()
				if ierr != nil {
					break
				}
				switch tok := inner.(type) {
				case xml.StartElement:
					depth++
					if tok.Name.Local == "ColumnReference" {
						for _, a := range tok.Attr {
							if a.Name.Local == "Column" {
								cols = append(cols, strings.Trim(a.Value, "[]"))
							}
						}
					}
				case xml.EndElement:
					depth--
				}
			}
			op.OutputColumns = len(cols)
			op.OutputColNames = cols
		case "Predicate":
			if op.Predicate == "" {
				op.Predicate = extractFirstScalarString(dec, t)
			}
		case "SeekPredicates":
			if op.SeekPredicate == "" {
				op.SeekPredicate = extractFirstScalarString(dec, t)
			}
		case "Warnings":
			var warn xmlWarningNode
			if err2 := dec.DecodeElement(&warn, &t); err2 == nil {
				if len(warn.ColumnsNoStats) > 0 || len(warn.SpillToTempDb) > 0 || len(warn.NoJoinPredicate) > 0 {
					op.HasWarnings = true
				}
				for _, c := range warn.ColumnsNoStats {
					op.WarningTexts = append(op.WarningTexts, "No stats: "+c.Table+"."+c.Column)
				}
				for _, sp := range warn.SpillToTempDb {
					op.WarningTexts = append(op.WarningTexts, "Spill to TempDB (level "+strconv.Itoa(sp.SpillLevel)+")")
					op.HasWarnings = true
				}
				for range warn.NoJoinPredicate {
					op.WarningTexts = append(op.WarningTexts, "No join predicate")
					op.HasWarnings = true
				}
			}
		}
	}
	return children
}

func convertRelOp(x *xmlRelOp, totalCost float64) *RelOp {
	if x == nil {
		return nil
	}
	op := &RelOp{
		NodeID:        x.NodeID,
		PhysicalOp:    x.PhysicalOp,
		LogicalOp:     x.LogicalOp,
		EstimatedCost: x.SubtreeCost,
		EstimatedRows: x.EstRows,
		EstRowsRead:   x.EstRowsRead,
		EstimateIO:    x.EstimateIO,
		EstimateCPU:   x.EstimateCPU,
		AvgRowSize:    x.AvgRowSize,
		TableCardinality: x.TableCard,
		EstRebinds:    x.EstRebinds,
		EstRewinds:    x.EstRewinds,
		ExecutionMode: x.ExecMode,
	}
	if x.ParallelStr == "1" || x.ParallelStr == "true" {
		op.Parallel = true
	}
	if totalCost > 0 {
		// Use the operator's own cost (IO + CPU), not the subtree cost.
		// SubtreeCost is cumulative (includes all descendants), which makes every
		// non-leaf node look expensive and the root always 100%.
		op.CostPercent = (x.EstimateIO + x.EstimateCPU) / totalCost * 100
	}

	// Single pass: extract extras AND find child RelOps at once
	var childXMLOps []*xmlRelOp
	if len(x.InnerXML) > 0 {
		childXMLOps = parseRelOpInnerXML(op, x.InnerXML)
	}
	for _, c := range childXMLOps {
		if child := convertRelOp(c, totalCost); child != nil {
			op.Children = append(op.Children, child)
		}
	}
	return op
}

func CountNodes(op *RelOp) int {
	if op == nil {
		return 0
	}
	n := 1
	for _, c := range op.Children {
		n += CountNodes(c)
	}
	return n
}

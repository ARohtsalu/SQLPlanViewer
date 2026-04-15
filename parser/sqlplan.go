package parser

import (
	"bytes"
	"encoding/xml"
	"os"
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
	Children      []*RelOp
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
	InnerXML    []byte  `xml:",innerxml"`
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

			// Include all statements (even DDL/SET with no RelOp) so tabs are complete
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
	}
	if totalCost > 0 {
		op.CostPercent = x.SubtreeCost / totalCost * 100
	}
	for _, c := range findDirectRelOps(x.InnerXML) {
		if child := convertRelOp(c, totalCost); child != nil {
			op.Children = append(op.Children, child)
		}
	}
	return op
}

func findDirectRelOps(data []byte) []*xmlRelOp {
	data = bytes.ReplaceAll(data,
		[]byte(`xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan"`),
		[]byte(""))

	var result []*xmlRelOp
	dec := xml.NewDecoder(bytes.NewReader(data))
	depth := 0

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "RelOp" && depth == 0 {
				var ro xmlRelOp
				if err := dec.DecodeElement(&ro, &t); err == nil {
					result = append(result, &ro)
				}
			} else {
				depth++
			}
		case xml.EndElement:
			if depth > 0 {
				depth--
			}
		}
	}
	return result
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

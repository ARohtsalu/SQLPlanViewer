package parser

import (
	"bytes"
	"encoding/xml"
	"os"
)

type QueryPlan struct {
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
	X, Y          float32 // layout positions
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

// --- XML structs ---

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
	Impact  float64        `xml:"Impact,attr"`
	Indexes []xmlMissingIdx `xml:"MissingIndex"`
}

type xmlMissingIdx struct {
	Database     string       `xml:"Database,attr"`
	Table        string       `xml:"Table,attr"`
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

// xmlRelOp captures attributes + raw inner XML for recursive child extraction
type xmlRelOp struct {
	NodeID      int     `xml:"NodeId,attr"`
	PhysicalOp  string  `xml:"PhysicalOp,attr"`
	LogicalOp   string  `xml:"LogicalOp,attr"`
	SubtreeCost float64 `xml:"EstimatedTotalSubtreeCost,attr"`
	EstRows     float64 `xml:"EstimateRows,attr"`
	InnerXML    []byte  `xml:",innerxml"`
}

func ParseSqlPlan(path string) (*QueryPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var root xmlShowPlanXML
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	qp := &QueryPlan{}

	for _, batch := range root.Batches {
		for _, stmt := range batch.Statements {
			qp.StatementText = stmt.StatementText
			qp.TotalCost = stmt.SubTreeCost
			qp.EstimatedRows = stmt.EstimatedRows

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
			return qp, nil
		}
	}

	return qp, nil
}

// convertRelOp converts xmlRelOp to RelOp, recursively finding child RelOps
// in the innerXML regardless of wrapper element names.
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

	// Find all direct <RelOp> children anywhere in the inner XML
	childOps := findDirectRelOps(x.InnerXML)
	for _, c := range childOps {
		child := convertRelOp(c, totalCost)
		if child != nil {
			op.Children = append(op.Children, child)
		}
	}
	return op
}

// findDirectRelOps finds <RelOp> elements that are direct children
// of the current element's content (one level deep in wrappers).
// We skip nested RelOps inside found RelOps.
func findDirectRelOps(data []byte) []*xmlRelOp {
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
				// Don't increment depth — DecodeElement consumed everything
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

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

func ParseSqlPlan(path string) (*QueryPlan, error) {
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

	// Find the most expensive statement that has an actual query plan
	var best *xmlStmt
	for i := range root.Batches {
		for j := range root.Batches[i].Statements {
			stmt := &root.Batches[i].Statements[j]
			if stmt.QueryPlan == nil || stmt.QueryPlan.RelOp == nil {
				continue
			}
			if best == nil || stmt.SubTreeCost > best.SubTreeCost {
				best = stmt
			}
		}
	}

	if best == nil {
		return &QueryPlan{}, nil
	}

	qp := &QueryPlan{
		StatementText: best.StatementText,
		TotalCost:     best.SubTreeCost,
		EstimatedRows: best.EstimatedRows,
	}

	for _, mig := range best.QueryPlan.MissingIndexes {
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

	if w := best.QueryPlan.Warnings; w != nil {
		for _, c := range w.ColumnsNoStats {
			qp.Warnings = append(qp.Warnings, Warning{
				Text: "No statistics: " + c.Table + "." + c.Column,
			})
		}
		for range w.SpillToTempDb {
			qp.Warnings = append(qp.Warnings, Warning{Text: "Spill to TempDB"})
		}
	}

	qp.RootOp = convertRelOp(best.QueryPlan.RelOp, qp.TotalCost)
	return qp, nil
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

// findDirectRelOps finds <RelOp> elements that are direct children
// within any wrapper elements, using a depth-aware token walk.
func findDirectRelOps(data []byte) []*xmlRelOp {
	// Strip namespace from inner XML too
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

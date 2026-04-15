package parser

import (
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

// --- XML structs for parsing ---

type xmlShowPlanXML struct {
	XMLName xml.Name   `xml:"ShowPlanXML"`
	Batches []xmlBatch `xml:"BatchSequence>Batch"`
}

type xmlBatch struct {
	Statements []xmlStmt `xml:"Statements>StmtSimple"`
}

type xmlStmt struct {
	StatementText     string       `xml:"StatementText,attr"`
	StatementSubCost  float64      `xml:"StatementSubTreeCost,attr"`
	EstimatedRows     float64      `xml:"StatementEstRows,attr"`
	QueryPlan         *xmlQPlan    `xml:"QueryPlan"`
}

type xmlQPlan struct {
	MissingIndexes []xmlMissingIndexGroup `xml:"MissingIndexes>MissingIndexGroup"`
	Warnings       *xmlWarnings           `xml:"Warnings"`
	RelOp          *xmlRelOp              `xml:"RelOp"`
}

type xmlMissingIndexGroup struct {
	Impact  float64          `xml:"Impact,attr"`
	Indexes []xmlMissingIdx  `xml:"MissingIndex"`
}

type xmlMissingIdx struct {
	Database  string          `xml:"Database,attr"`
	Schema    string          `xml:"Schema,attr"`
	Table     string          `xml:"Table,attr"`
	ColumnGroups []xmlColGroup `xml:"ColumnGroup"`
}

type xmlColGroup struct {
	Usage   string     `xml:"Usage,attr"`
	Columns []xmlCol   `xml:"Column"`
}

type xmlCol struct {
	Name string `xml:"Name,attr"`
}

type xmlWarnings struct {
	ColumnsNoStats []xmlColNoStats `xml:"ColumnsWithNoStatistics>ColumnReference"`
	SpillToTempDb  []xmlSpill      `xml:"SpillToTempDb"`
}

type xmlColNoStats struct {
	Table  string `xml:"Table,attr"`
	Column string `xml:"Column,attr"`
}

type xmlSpill struct {
	SpillLevel int `xml:"SpillLevel,attr"`
}

type xmlRelOp struct {
	NodeID        int       `xml:"NodeId,attr"`
	PhysicalOp    string    `xml:"PhysicalOp,attr"`
	LogicalOp     string    `xml:"LogicalOp,attr"`
	SubtreeCost   float64   `xml:"EstimatedTotalSubtreeCost,attr"`
	EstimatedRows float64   `xml:"EstimatedRows,attr"`
	// Children can be nested inside operator-specific elements; we collect all RelOp descendants
	InnerRelOps []xmlRelOp `xml:",any>RelOp"`
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
			qp.TotalCost = stmt.StatementSubCost
			qp.EstimatedRows = stmt.EstimatedRows

			if stmt.QueryPlan != nil {
				// Missing indexes
				for _, mig := range stmt.QueryPlan.MissingIndexes {
					for _, mi := range mig.Indexes {
						cols := collectColumns(mi.ColumnGroups)
						qp.MissingIndexes = append(qp.MissingIndexes, MissingIndex{
							Database: mi.Database,
							Table:    mi.Table,
							Columns:  cols,
							Impact:   mig.Impact,
						})
					}
				}

				// Warnings
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

				// Plan tree
				if stmt.QueryPlan.RelOp != nil {
					qp.RootOp = convertRelOp(stmt.QueryPlan.RelOp, qp.TotalCost)
				}
			}
			// Only first statement
			return qp, nil
		}
	}

	return qp, nil
}

func collectColumns(groups []xmlColGroup) string {
	result := ""
	for _, g := range groups {
		for i, c := range g.Columns {
			if i > 0 {
				result += ", "
			}
			result += c.Name
		}
	}
	return result
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
		EstimatedRows: x.EstimatedRows,
	}
	if totalCost > 0 {
		op.CostPercent = x.SubtreeCost / totalCost * 100
	}
	for i := range x.InnerRelOps {
		child := convertRelOp(&x.InnerRelOps[i], totalCost)
		if child != nil {
			op.Children = append(op.Children, child)
		}
	}
	return op
}

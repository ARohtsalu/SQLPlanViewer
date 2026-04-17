package ui

import (
	"strings"

	"fyne.io/fyne/v2"
)

// planIconName maps a PhysicalOp string to an icon filename (without .png).
// Ported from PlanViewer.Core/Services/PlanIconMapper.cs.
func planIconName(physicalOp string) string {
	switch physicalOp {
	// Join operators
	case "Adaptive Join":
		return "adaptive_join"
	case "Hash Match":
		return "hash_match"
	case "Merge Join":
		return "merge_join"
	case "Nested Loops":
		return "nested_loops"

	// Index operations
	case "Clustered Index Delete":
		return "clustered_index_delete"
	case "Clustered Index Insert":
		return "clustered_index_insert"
	case "Clustered Index Merge":
		return "clustered_index_merge"
	case "Clustered Index Scan":
		return "clustered_index_scan"
	case "Clustered Index Seek":
		return "clustered_index_seek"
	case "Clustered Index Update":
		return "clustered_index_update"
	case "Clustered Update":
		return "clustered_update"
	case "Index Delete":
		return "index_delete"
	case "Index Insert":
		return "index_insert"
	case "Index Scan":
		return "index_scan"
	case "Index Seek":
		return "index_seek"
	case "Index Spool", "Eager Index Spool", "Lazy Index Spool":
		return "index_spool"
	case "Index Update":
		return "index_update"

	// Columnstore
	case "Columnstore Index Delete":
		return "columnstore_index_delete"
	case "Columnstore Index Insert":
		return "columnstore_index_insert"
	case "Columnstore Index Merge":
		return "columnstore_index_merge"
	case "Columnstore Index Scan":
		return "columnstore_index_scan"
	case "Columnstore Index Update":
		return "columnstore_index_update"

	// Scan operators
	case "Table Scan":
		return "table_scan"
	case "Constant Scan":
		return "constant_scan"

	// Table DML
	case "Table Delete":
		return "table_delete"
	case "Table Insert":
		return "table_insert"
	case "Table Merge":
		return "table_merge"
	case "Table Update":
		return "table_update"

	// Lookup
	case "Key Lookup", "Bookmark Lookup":
		return "bookmark_lookup"
	case "RID Lookup":
		return "rid_lookup"

	// Aggregation
	case "Stream Aggregate":
		return "stream_aggregate"
	case "Window Aggregate":
		return "window_aggregate"

	// Scalar / compute
	case "Compute Scalar":
		return "compute_scalar"
	case "Filter":
		return "filter"
	case "Assert":
		return "assert"

	// Sort / top
	case "Sort", "Distinct":
		return "sort"
	case "Top":
		return "top"

	// Spool
	case "Table Spool", "Eager Table Spool", "Lazy Table Spool",
		"Window Spool", "Eager Spool", "Lazy Spool", "Spool":
		return "table_spool"
	case "Row Count Spool", "Eager Row Count Spool", "Lazy Row Count Spool":
		return "row_count_spool"

	// Set operations
	case "Concatenation":
		return "concatenation"
	case "Union":
		return "union"
	case "Union All":
		return "union_all"

	// Parallelism
	case "Parallelism", "Distribute Streams", "Gather Streams", "Repartition Streams":
		return "parallelism"

	// Remote
	case "Remote Query":
		return "remote_query"
	case "Remote Scan":
		return "remote_scan"

	// Misc
	case "Bitmap":
		return "bitmap"
	case "Segment":
		return "segment"
	case "Sequence":
		return "sequence"
	case "Sequence Project":
		return "sequence_project"
	case "Split":
		return "split"
	case "Table-valued function":
		return "table_valued_function"
	case "Result":
		return "result"
	case "Aggregate":
		return "aggregate"
	}

	// Fallback: spaces → underscores, lowercase
	fallback := strings.ToLower(strings.ReplaceAll(physicalOp, " ", "_"))
	return fallback
}

// iconResCache caches loaded resources to avoid re-reading the embed FS.
var iconResCache = map[string]fyne.Resource{}

// loadIconResource returns the embedded PNG resource for the given icon name.
// Falls back to iterator_catch_all.png if not found.
func loadIconResource(iconName string) fyne.Resource {
	if res, ok := iconResCache[iconName]; ok {
		return res
	}
	data, err := planIconsFS.ReadFile("planicons/" + iconName + ".png")
	if err != nil {
		data, err = planIconsFS.ReadFile("planicons/iterator_catch_all.png")
		if err != nil {
			iconResCache[iconName] = nil
			return nil
		}
	}
	res := fyne.NewStaticResource(iconName+".png", data)
	iconResCache[iconName] = res
	return res
}

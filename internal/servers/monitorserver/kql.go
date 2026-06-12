package monitorserver

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"localaz.dev/internal/stores/monitorstore"
)

// resultTable is the intermediate (and final) shape produced by evaluating a
// KQL pipeline: an ordered list of column names and the rows that survived the
// pipeline. An empty columns slice means "infer the columns from the rows".
type resultTable struct {
	columns []string
	rows    []monitorstore.Row
}

// evalKQL evaluates the supported subset of KQL against the store and returns
// the resulting table. The supported grammar is:
//
//	TableName
//	| where <col> <op> <literal> [and|or <col> <op> <literal> ...]
//	| project <col> [, <col> ...]
//	| sort by <col> [asc|desc]   (also spelled "order by")
//	| take <n>                   (also spelled "limit")
//	| count
//
// where <op> is one of == != < <= > >= and literals are single/double quoted
// strings, numbers, or the bare words true/false. Anything outside this subset
// (joins, OData functions, summarize, parentheses, etc.) is rejected.
func evalKQL(store *monitorstore.Store, query string) (resultTable, error) {
	stages := splitPipeline(query)
	if len(stages) == 0 {
		return resultTable{}, fmt.Errorf("empty query")
	}

	name := strings.TrimSpace(stages[0])
	if name == "" {
		return resultTable{}, fmt.Errorf("missing table name")
	}
	rows, ok := store.Rows(name)
	if !ok {
		return resultTable{}, fmt.Errorf("unknown table %q", name)
	}

	tbl := resultTable{rows: rows}
	for _, stage := range stages[1:] {
		var err error
		tbl, err = applyStage(tbl, stage)
		if err != nil {
			return resultTable{}, err
		}
	}
	return tbl, nil
}

// splitPipeline splits a query on the top-level '|' separators.
func splitPipeline(query string) []string {
	parts := strings.Split(query, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// applyStage applies a single pipeline operator to the working table.
func applyStage(tbl resultTable, stage string) (resultTable, error) {
	fields := strings.Fields(stage)
	if len(fields) == 0 {
		return tbl, nil
	}
	op := strings.ToLower(fields[0])
	rest := strings.TrimSpace(stage[len(fields[0]):])

	switch op {
	case "where", "filter":
		return applyWhere(tbl, rest)
	case "project":
		return applyProject(tbl, rest)
	case "take", "limit":
		return applyTake(tbl, rest)
	case "count":
		return applyCount(tbl), nil
	case "sort", "order":
		return applySort(tbl, fields)
	default:
		return resultTable{}, fmt.Errorf("unsupported operator %q", fields[0])
	}
}

// applyWhere keeps the rows that satisfy the predicate expression.
func applyWhere(tbl resultTable, expr string) (resultTable, error) {
	pred, err := parsePredicate(expr)
	if err != nil {
		return resultTable{}, err
	}
	kept := tbl.rows[:0:0]
	for _, r := range tbl.rows {
		ok, err := pred.eval(r)
		if err != nil {
			return resultTable{}, err
		}
		if ok {
			kept = append(kept, r)
		}
	}
	tbl.rows = kept
	return tbl, nil
}

// applyProject restricts (and orders) the columns to those named.
func applyProject(tbl resultTable, list string) (resultTable, error) {
	cols := splitCSV(list)
	if len(cols) == 0 {
		return resultTable{}, fmt.Errorf("project requires at least one column")
	}
	out := make([]monitorstore.Row, len(tbl.rows))
	for i, r := range tbl.rows {
		nr := make(monitorstore.Row, len(cols))
		for _, c := range cols {
			if v, ok := r[c]; ok {
				nr[c] = v
			} else {
				nr[c] = nil
			}
		}
		out[i] = nr
	}
	tbl.columns = cols
	tbl.rows = out
	return tbl, nil
}

// applyTake keeps the first n rows.
func applyTake(tbl resultTable, arg string) (resultTable, error) {
	n, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil || n < 0 {
		return resultTable{}, fmt.Errorf("take requires a non-negative integer")
	}
	if n < len(tbl.rows) {
		tbl.rows = tbl.rows[:n]
	}
	return tbl, nil
}

// applyCount collapses the table to a single Count row.
func applyCount(tbl resultTable) resultTable {
	return resultTable{
		columns: []string{"Count"},
		rows:    []monitorstore.Row{{"Count": float64(len(tbl.rows))}},
	}
}

// applySort orders the rows by a single column. The stage text is "sort by col
// [asc|desc]" or "order by col [asc|desc]".
func applySort(tbl resultTable, fields []string) (resultTable, error) {
	if len(fields) < 3 || strings.ToLower(fields[1]) != "by" {
		return resultTable{}, fmt.Errorf("sort must be written as 'sort by <col> [asc|desc]'")
	}
	col := strings.TrimSuffix(fields[2], ",")
	desc := false
	if len(fields) >= 4 {
		switch strings.ToLower(fields[3]) {
		case "asc":
			desc = false
		case "desc":
			desc = true
		default:
			return resultTable{}, fmt.Errorf("sort direction must be asc or desc")
		}
	} else {
		// KQL defaults to descending order.
		desc = true
	}
	sort.SliceStable(tbl.rows, func(i, j int) bool {
		less := compareValues(tbl.rows[i][col], tbl.rows[j][col]) < 0
		if desc {
			return !less && compareValues(tbl.rows[i][col], tbl.rows[j][col]) != 0
		}
		return less
	})
	return tbl, nil
}

// splitCSV splits a comma-separated list and trims each element.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

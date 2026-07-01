package plugin

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Aggregation functions supported by grouped item queries. These map to
// monday.com's server-side `aggregate` query functions.
const (
	aggCount         = "count"          // COUNT_ITEMS (no column)
	aggCountDistinct = "count_distinct" // COUNT_DISTINCT (column)
	aggSum           = "sum"            // SUM (column)
	aggAvg           = "avg"            // AVERAGE (column)
	aggMin           = "min"            // MIN (column)
	aggMax           = "max"            // MAX (column)
)

// validAggregation reports whether agg is a supported aggregation.
func validAggregation(agg string) bool {
	switch agg {
	case aggCount, aggCountDistinct, aggSum, aggAvg, aggMin, aggMax:
		return true
	default:
		return false
	}
}

// mondayAggregateFunction maps our aggregation name to monday's `aggregate`
// function enum and whether that function needs a column argument.
func mondayAggregateFunction(agg string) (fn string, needsColumn bool) {
	switch agg {
	case aggCountDistinct:
		return "COUNT_DISTINCT", true
	case aggSum:
		return "SUM", true
	case aggAvg:
		return "AVERAGE", true
	case aggMin:
		return "MIN", true
	case aggMax:
		return "MAX", true
	default: // aggCount
		return "COUNT_ITEMS", false
	}
}

// aliasResult is the stable alias for the aggregation result entry. The group
// column entry, by contrast, MUST be aliased to its own column_id so monday ties
// the selected column to the `group_by` clause (see buildAggregateQuery).
const (
	aliasResult = "result_value"
	// groupLimit caps the number of distinct groups returned by group_by.
	groupLimit = 500
)

// buildAggregateQuery builds the monday.com `aggregate` GraphQL document and its
// variables for a single board. groupBy and aggCol are column ids. filterRules is
// the optional ItemsQuery rules array (same shape as items_page filtering).
//
// The query selects the aggregation result (alias "result_value") and, when
// grouping, the group column value (alias "group_value"), grouped by groupBy.
func buildAggregateQuery(boardID, groupBy, agg, aggCol string, filterRules []any) (string, map[string]any, error) {
	if strings.TrimSpace(boardID) == "" {
		return "", nil, fmt.Errorf("a board id is required for aggregation")
	}
	if !validAggregation(agg) {
		agg = aggCount
	}
	fn, needsCol := mondayAggregateFunction(agg)
	aggCol = strings.TrimSpace(aggCol)
	if needsCol && aggCol == "" {
		return "", nil, fmt.Errorf("aggregation %q requires a value column", agg)
	}
	groupBy = strings.TrimSpace(groupBy)

	// Build the result-function select element.
	var resultSelect string
	if needsCol {
		resultSelect = `{ type: FUNCTION, function: { function: ` + fn +
			`, params: [{ type: COLUMN, column: { column_id: ` + jsonString(aggCol) +
			` }, as: ` + jsonString(aggCol) + ` }] }, as: ` + jsonString(aliasResult) + ` }`
	} else {
		resultSelect = `{ type: FUNCTION, function: { function: ` + fn + ` }, as: ` + jsonString(aliasResult) + ` }`
	}

	selects := []string{resultSelect}
	groupByClause := ""
	if groupBy != "" {
		// The group column's `as` MUST equal its column_id so monday associates
		// the selected column with the `group_by` clause; otherwise it returns a
		// single ungrouped result with a null group value.
		selects = append(selects,
			`{ type: COLUMN, column: { column_id: `+jsonString(groupBy)+` }, as: `+jsonString(groupBy)+` }`)
		groupByClause = `, group_by: [{ column_id: ` + jsonString(groupBy) + `, limit: ` + fmt.Sprintf("%d", groupLimit) + ` }]`
	}

	filterClause := ""
	variables := map[string]any{}
	if len(filterRules) > 0 {
		filterClause = `, query: $itemsQuery`
		variables["itemsQuery"] = map[string]any{"rules": filterRules}
	}

	queryVarDecl := ""
	if filterClause != "" {
		queryVarDecl = "($itemsQuery: ItemsQuery)"
	}

	doc := `query Aggregate` + queryVarDecl + ` {
  aggregate(query: {
    from: { type: TABLE, id: ` + jsonString(boardID) + ` },
    select: [
      ` + strings.Join(selects, ",\n      ") + `
    ]` + groupByClause + filterClause + `
  }) {
    results {
      entries {
        alias
        value {
          ... on AggregateBasicAggregationResult { result }
          ... on AggregateGroupByResult { value }
        }
      }
    }
  }
}`
	return doc, variables, nil
}

// jsonString renders s as a JSON/GraphQL string literal (quoted, escaped).
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// aggregateEntry is one alias/value pair within a result set. The `value` object
// is kept raw so we can flexibly extract whichever field monday populates
// (`result` for basic aggregations, `value` for group-by results, and any
// nested/scalar variants across API versions).
type aggregateEntry struct {
	Alias string          `json:"alias"`
	Value json.RawMessage `json:"value"`
}

// entryScalar pulls a usable scalar out of an aggregate entry's `value` object.
// It prefers the `result` field, then `value`, then any single scalar field, and
// finally treats the whole value as a scalar if it is itself a scalar.
func entryScalar(raw json.RawMessage) any {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	// Object form: { "result": ... } or { "value": ... } (or other fields).
	if trimmed[0] == '{' {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err == nil {
			for _, key := range []string{"result", "value"} {
				if v, ok := obj[key]; ok {
					if s := decodeAggregateScalar(v); s != nil {
						return s
					}
				}
			}
			// Fall back to the first scalar field present.
			for _, v := range obj {
				if s := decodeAggregateScalar(v); s != nil {
					return s
				}
			}
			return nil
		}
	}
	// Scalar form: the value object is itself a scalar.
	return decodeAggregateScalar(raw)
}

// aggregateResponse is the shape of the `aggregate` query data.
type aggregateResponse struct {
	Aggregate struct {
		Results []struct {
			Entries []aggregateEntry `json:"entries"`
		} `json:"results"`
	} `json:"aggregate"`
}

// parseAggregateResults converts the aggregate response into flat records. Each
// result set becomes one row: the group column (keyed by groupCol) and the
// aggregation result (keyed by resultCol). groupAlias is the alias monday
// returns for the group column entry (its column_id); pass "" when not grouping.
// When not grouping, a single row with just the result is returned.
func parseAggregateResults(data json.RawMessage, groupCol, groupAlias, resultCol string) ([]map[string]any, error) {
	var resp aggregateResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed parsing aggregate response: %w", err)
	}
	grouping := strings.TrimSpace(groupAlias) != ""
	rows := make([]map[string]any, 0, len(resp.Aggregate.Results))
	for _, set := range resp.Aggregate.Results {
		row := map[string]any{}
		for _, e := range set.Entries {
			if e.Alias == aliasResult {
				row[resultCol] = entryScalar(e.Value)
				continue
			}
			if grouping && e.Alias == groupAlias {
				v := entryScalar(e.Value)
				if v == nil || v == "" {
					row[groupCol] = emptyGroupLabel
				} else {
					row[groupCol] = v
				}
			}
		}
		if _, ok := row[resultCol]; !ok {
			row[resultCol] = nil
		}
		if grouping {
			if _, ok := row[groupCol]; !ok {
				row[groupCol] = emptyGroupLabel
			}
		}
		rows = append(rows, row)
	}
	sortAggregated(rows, groupCol, resultCol)
	return rows, nil
}

// decodeAggregateScalar decodes a raw JSON aggregate value (number, string, bool
// or null) into a Go scalar suitable for the frame builder.
func decodeAggregateScalar(raw json.RawMessage) any {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	var v any
	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil
	}
	if n, ok := v.(json.Number); ok {
		if f, err := n.Float64(); err == nil {
			return f
		}
		return n.String()
	}
	return v
}

// emptyGroupLabel is used as the group value when the grouped column has no value
// for a result set.
const emptyGroupLabel = "(empty)"

// aggregationColumnName builds a human-readable result column name, e.g.
// "count", "sum(numbers)", "count_distinct(owner)".
func aggregationColumnName(agg, aggCol string) string {
	if agg == aggCount {
		return "count"
	}
	if aggCol == "" {
		return agg
	}
	return agg + "(" + aggCol + ")"
}

// sortAggregated sorts rows by the numeric result descending, then by group
// value ascending. Nil results sort last.
func sortAggregated(rows []map[string]any, groupCol, resultCol string) {
	sort.SliceStable(rows, func(i, j int) bool {
		fi, oki := toFloat(rows[i][resultCol])
		fj, okj := toFloat(rows[j][resultCol])
		if oki != okj {
			return oki // non-nil before nil
		}
		if oki && okj && fi != fj {
			return fi > fj // larger result first
		}
		si, _ := toString(rows[i][groupCol])
		sj, _ := toString(rows[j][groupCol])
		return si < sj
	})
}

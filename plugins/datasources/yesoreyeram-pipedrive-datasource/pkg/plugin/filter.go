package plugin

import (
	"strconv"
	"strings"
)

// applyFilters applies client-side filtering to records based on filter groups.
// Pipedrive's list API supports only a limited set of query params
// (status, user_id, stage_id, pipeline_id) for deals. For other fields,
// we filter client-side.
func applyFilters(records []map[string]any, groups []FilterGroup) []map[string]any {
	if len(groups) == 0 {
		return records
	}

	out := make([]map[string]any, 0, len(records))

	for _, rec := range records {
		if matchFilterGroups(rec, groups) {
			out = append(out, rec)
		}
	}

	return out
}

// matchFilterGroups returns true if the record matches the filter groups.
// Filters within a group are AND'd; groups are OR'd.
func matchFilterGroups(rec map[string]any, groups []FilterGroup) bool {
	if len(groups) == 0 {
		return true
	}
	for _, group := range groups {
		if matchFilterGroup(rec, group) {
			return true
		}
	}
	return false
}

func matchFilterGroup(rec map[string]any, group FilterGroup) bool {
	if len(group.Filters) == 0 {
		return true
	}
	for _, f := range group.Filters {
		fieldVal, ok := rec[f.Field]
		if !ok {
			return false
		}
		if !matchFilter(fieldVal, f.Operator, f.Value) {
			return false
		}
	}
	return true
}

func matchFilter(fieldVal any, operator, value string) bool {
	val := strings.ToLower(strings.TrimSpace(value))
	fieldStr, isStr := fieldVal.(string)
	if !isStr {
		if fieldVal == nil {
			fieldStr = ""
		} else {
			fieldStr, _ = toString(fieldVal)
		}
	}
	fieldStr = strings.ToLower(strings.TrimSpace(fieldStr))

	switch strings.ToUpper(strings.TrimSpace(operator)) {
	case "EQ":
		return fieldStr == val
	case "NEQ":
		return fieldStr != val
	case "LIKE":
		return strings.Contains(fieldStr, val)
	case "NOT_LIKE":
		return !strings.Contains(fieldStr, val)
	case "GT":
		return compareNumeric(fieldVal, value) > 0
	case "GTE":
		return compareNumeric(fieldVal, value) >= 0
	case "LT":
		return compareNumeric(fieldVal, value) < 0
	case "LTE":
		return compareNumeric(fieldVal, value) <= 0
	default:
		return fieldStr == val
	}
}

func compareNumeric(fieldVal any, value string) int {
	var f1, f2 float64
	switch v := fieldVal.(type) {
	case float64:
		f1 = v
	case string:
		f1, _ = strconv.ParseFloat(v, 64)
	case int:
		f1 = float64(v)
	case int64:
		f1 = float64(v)
	default:
		return -1
	}
	f2, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return -1
	}
	if f1 < f2 {
		return -1
	}
	if f1 > f2 {
		return 1
	}
	return 0
}

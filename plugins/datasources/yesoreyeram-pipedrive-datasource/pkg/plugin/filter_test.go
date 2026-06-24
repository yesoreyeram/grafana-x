package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyFilters(t *testing.T) {
	records := []map[string]any{
		{"id": float64(1), "title": "Big Deal", "value": float64(10000), "status": "open"},
		{"id": float64(2), "title": "Small Deal", "value": float64(100), "status": "won"},
	}

	t.Run("no filters returns all", func(t *testing.T) {
		require.Len(t, applyFilters(records, nil), 2)
	})

	t.Run("EQ filter", func(t *testing.T) {
		groups := []FilterGroup{{Filters: []Filter{{Field: "title", Operator: "EQ", Value: "Big Deal"}}}}
		result := applyFilters(records, groups)
		require.Len(t, result, 1)
		require.Equal(t, "Big Deal", result[0]["title"])
	})

	t.Run("GT numeric filter", func(t *testing.T) {
		groups := []FilterGroup{{Filters: []Filter{{Field: "value", Operator: "GT", Value: "1000"}}}}
		result := applyFilters(records, groups)
		require.Len(t, result, 1)
		require.EqualValues(t, 1, result[0]["id"])
	})

	t.Run("LIKE filter", func(t *testing.T) {
		groups := []FilterGroup{{Filters: []Filter{{Field: "title", Operator: "LIKE", Value: "deal"}}}}
		require.Len(t, applyFilters(records, groups), 2)
	})

	t.Run("AND within group", func(t *testing.T) {
		groups := []FilterGroup{{Filters: []Filter{
			{Field: "status", Operator: "EQ", Value: "open"},
			{Field: "value", Operator: "GT", Value: "1000"},
		}}}
		result := applyFilters(records, groups)
		require.Len(t, result, 1)
		require.EqualValues(t, 1, result[0]["id"])
	})

	t.Run("OR across groups", func(t *testing.T) {
		groups := []FilterGroup{
			{Filters: []Filter{{Field: "status", Operator: "EQ", Value: "open"}}},
			{Filters: []Filter{{Field: "status", Operator: "EQ", Value: "won"}}},
		}
		require.Len(t, applyFilters(records, groups), 2)
	})
}

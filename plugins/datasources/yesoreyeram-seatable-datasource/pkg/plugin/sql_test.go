package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSelectSQL_AllColumns(t *testing.T) {
	sql, params := BuildSelectSQL(QueryModel{TableName: "My Table"})
	require.Equal(t, "SELECT * FROM `My Table`", sql)
	require.Nil(t, params)
}

func TestBuildSelectSQL_ProjectedColumns(t *testing.T) {
	sql, _ := BuildSelectSQL(QueryModel{TableName: "T", Fields: "Name, Age"})
	require.Equal(t, "SELECT `Name`, `Age` FROM `T`", sql)
}

func TestBuildSelectSQL_WhereAndOrder(t *testing.T) {
	q := QueryModel{
		TableName: "T",
		filter:    group("and", cond("Plan", "eq", "pro"), cond("Age", "gt", "21")),
		sortItems: []SortItem{{Field: "Age", Direction: "desc"}, {Field: "Name", Direction: "asc"}},
	}
	sql, params := BuildSelectSQL(q)
	require.Equal(t, "SELECT * FROM `T` WHERE (`Plan` = ? AND `Age` > ?) ORDER BY `Age` DESC, `Name` ASC", sql)
	require.Equal(t, []any{"pro", float64(21)}, params)
}

func TestBuildSelectSQL_SkipsFieldlessSort(t *testing.T) {
	q := QueryModel{TableName: "T", sortItems: []SortItem{{Field: "", Direction: "asc"}, {Field: "Age", Direction: "asc"}}}
	sql, _ := BuildSelectSQL(q)
	require.Equal(t, "SELECT * FROM `T` ORDER BY `Age` ASC", sql)
}

func TestBuildCountSQL(t *testing.T) {
	sql, params := BuildCountSQL(QueryModel{TableName: "T"})
	require.Equal(t, "SELECT COUNT(*) FROM `T`", sql)
	require.Nil(t, params)
}

func TestBuildCountSQL_WithFilter(t *testing.T) {
	q := QueryModel{TableName: "T", filter: group("and", cond("Done", "eq", "true"))}
	sql, params := BuildCountSQL(q)
	require.Equal(t, "SELECT COUNT(*) FROM `T` WHERE `Done` = ?", sql)
	require.Equal(t, []any{"true"}, params)
}

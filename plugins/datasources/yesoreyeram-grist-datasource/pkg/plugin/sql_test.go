package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSelectSQL_StarNoClauses(t *testing.T) {
	sql, args := BuildSelectSQL(QueryModel{TableID: "Users"})
	require.Equal(t, `SELECT * FROM "Users"`, sql)
	require.Nil(t, args)
}

func TestBuildSelectSQL_FieldsProjection(t *testing.T) {
	sql, args := BuildSelectSQL(QueryModel{TableID: "Users", Fields: "Name, Age"})
	require.Equal(t, `SELECT "Name", "Age" FROM "Users"`, sql)
	require.Nil(t, args)
}

func TestBuildSelectSQL_WhereOrderLimit(t *testing.T) {
	q := QueryModel{
		TableID:   "Users",
		Limit:     10,
		filter:    groupRef("and", cond("Age", "gte", "18")),
		sortItems: []SortItem{{Field: "Age", Direction: "desc"}, {Field: "Name", Direction: "asc"}},
	}
	sql, args := BuildSelectSQL(q)
	require.Equal(t, `SELECT * FROM "Users" WHERE "Age" >= ? ORDER BY "Age" DESC, "Name" ASC LIMIT 10`, sql)
	require.Equal(t, []any{float64(18)}, args)
}

func TestBuildCountSQL(t *testing.T) {
	sql, args := BuildCountSQL(QueryModel{TableID: "Users"})
	require.Equal(t, `SELECT COUNT(*) AS count FROM "Users"`, sql)
	require.Nil(t, args)
}

func TestBuildCountSQL_WithWhere(t *testing.T) {
	q := QueryModel{TableID: "Users", filter: groupRef("and", cond("Plan", "eq", "pro"))}
	sql, args := BuildCountSQL(q)
	require.Equal(t, `SELECT COUNT(*) AS count FROM "Users" WHERE "Plan" = ?`, sql)
	require.Equal(t, []any{"pro"}, args)
}

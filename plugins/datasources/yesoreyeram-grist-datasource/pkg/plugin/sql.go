package plugin

import (
	"strconv"
	"strings"
)

// BuildSelectSQL assembles the read-only SELECT statement for a record query
// that must run via the SQL endpoint (rich filter, OR logic, or a column
// projection). It returns the statement and the ordered arguments for the WHERE
// placeholders.
//
//	SELECT <cols> FROM "table" [WHERE ...] [ORDER BY ...] [LIMIT n]
func BuildSelectSQL(q QueryModel) (string, []any) {
	cols := "*"
	if fields := splitFields(q.Fields); len(fields) > 0 {
		quoted := make([]string, len(fields))
		for i, f := range fields {
			quoted[i] = identifier(f)
		}
		cols = strings.Join(quoted, ", ")
	}

	var b strings.Builder
	b.WriteString("SELECT ")
	b.WriteString(cols)
	b.WriteString(" FROM ")
	b.WriteString(identifier(q.TableID))

	where, args := BuildWhere(q.filter)
	if where != "" {
		b.WriteString(" WHERE ")
		b.WriteString(where)
	}
	if orderBy := buildOrderBy(q.sortItems); orderBy != "" {
		b.WriteString(" ORDER BY ")
		b.WriteString(orderBy)
	}
	if q.Limit > 0 {
		b.WriteString(" LIMIT ")
		b.WriteString(strconv.Itoa(q.Limit))
	}
	return b.String(), args
}

// BuildCountSQL assembles the COUNT(*) statement for a count query. Grist has no
// count endpoint, so counts are derived from a SQL COUNT(*).
//
//	SELECT COUNT(*) AS count FROM "table" [WHERE ...]
func BuildCountSQL(q QueryModel) (string, []any) {
	var b strings.Builder
	b.WriteString("SELECT COUNT(*) AS count FROM ")
	b.WriteString(identifier(q.TableID))

	where, args := BuildWhere(q.filter)
	if where != "" {
		b.WriteString(" WHERE ")
		b.WriteString(where)
	}
	return b.String(), args
}

// buildOrderBy renders the ORDER BY column list from the structured sort items.
// Items without a field are skipped; an empty result returns "".
func buildOrderBy(items []SortItem) string {
	parts := make([]string, 0, len(items))
	for _, s := range items {
		field := strings.TrimSpace(s.Field)
		if field == "" {
			continue
		}
		dir := "ASC"
		if strings.EqualFold(s.Direction, "desc") {
			dir = "DESC"
		}
		parts = append(parts, identifier(field)+" "+dir)
	}
	return strings.Join(parts, ", ")
}

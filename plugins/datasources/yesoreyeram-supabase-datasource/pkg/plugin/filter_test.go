package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func cond(field, op, value string) FilterNode {
	return FilterNode{Kind: "condition", Field: field, Op: op, Value: value}
}

func group(connector string, children ...FilterNode) FilterNode {
	return FilterNode{Kind: "group", Connector: connector, Children: children}
}

func TestBuildParams_Empty(t *testing.T) {
	require.Nil(t, BuildParams(group("and")))
}

func TestBuildParams_NonGroupRoot(t *testing.T) {
	require.Nil(t, BuildParams(cond("a", "eq", "1")))
}

func TestBuildParams_SimpleEq(t *testing.T) {
	params := BuildParams(group("and", cond("name", "eq", "Alice")))
	require.Equal(t, []Param{{Key: "name", Value: "eq.Alice"}}, params)
}

func TestBuildParams_MultipleAnd(t *testing.T) {
	params := BuildParams(group("and",
		cond("age", "gt", "18"),
		cond("status", "eq", "active"),
	))
	require.Equal(t, []Param{
		{Key: "age", Value: "gt.18"},
		{Key: "status", Value: "eq.active"},
	}, params)
}

// TestBuildParams_OrGroup is the critical correctness test: PostgREST `or=(...)`
// elements MUST be field-qualified (`status.eq.draft`), not bare (`eq.draft`).
func TestBuildParams_OrGroup(t *testing.T) {
	params := BuildParams(group("or",
		cond("status", "eq", "draft"),
		cond("status", "eq", "archived"),
	))
	require.Equal(t, []Param{
		{Key: "or", Value: "(status.eq.draft,status.eq.archived)"},
	}, params)
}

func TestBuildParams_OrGroupAcrossFields(t *testing.T) {
	params := BuildParams(group("or",
		cond("age", "gt", "30"),
		cond("age", "lt", "18"),
	))
	require.Equal(t, []Param{
		{Key: "or", Value: "(age.gt.30,age.lt.18)"},
	}, params)
}

func TestBuildParams_IsNull(t *testing.T) {
	params := BuildParams(group("and", cond("deleted_at", "is.null", "")))
	require.Equal(t, []Param{{Key: "deleted_at", Value: "is.null"}}, params)
}

func TestBuildParams_LegacyIsToken(t *testing.T) {
	// The legacy unary "is" token (no target) defaults to is.null.
	params := BuildParams(group("and", cond("deleted_at", "is", "")))
	require.Equal(t, []Param{{Key: "deleted_at", Value: "is.null"}}, params)
}

func TestBuildParams_IsNotNull(t *testing.T) {
	params := BuildParams(group("and", cond("email", "not.is.null", "")))
	require.Equal(t, []Param{{Key: "email", Value: "not.is.null"}}, params)
}

func TestBuildParams_LegacyNotIsToken(t *testing.T) {
	params := BuildParams(group("and", cond("email", "not.is", "")))
	require.Equal(t, []Param{{Key: "email", Value: "not.is.null"}}, params)
}

func TestBuildParams_IsTrueFalse(t *testing.T) {
	require.Equal(t, []Param{{Key: "active", Value: "is.true"}},
		BuildParams(group("and", cond("active", "is.true", ""))))
	require.Equal(t, []Param{{Key: "active", Value: "is.false"}},
		BuildParams(group("and", cond("active", "is.false", ""))))
}

func TestBuildParams_Negation(t *testing.T) {
	require.Equal(t, []Param{{Key: "status", Value: "not.eq.active"}},
		BuildParams(group("and", cond("status", "not.eq", "active"))))
	require.Equal(t, []Param{{Key: "name", Value: "not.like.A*"}},
		BuildParams(group("and", cond("name", "not.like", "A*"))))
}

func TestBuildParams_In(t *testing.T) {
	params := BuildParams(group("and", cond("role", "in", "admin,user")))
	require.Equal(t, []Param{{Key: "role", Value: "in.(admin,user)"}}, params)
}

func TestBuildParams_InQuotesReservedTokens(t *testing.T) {
	// A token containing a reserved character is double-quoted within the list.
	params := BuildParams(group("and", cond("city", "in", "Paris,New (York)")))
	require.Equal(t, []Param{{Key: "city", Value: `in.(Paris,"New (York)")`}}, params)
}

func TestBuildParams_InEmptySkipped(t *testing.T) {
	require.Nil(t, BuildParams(group("and", cond("role", "in", " , "))))
}

func TestBuildParams_LikeIlikePassThrough(t *testing.T) {
	require.Equal(t, []Param{{Key: "name", Value: "like.A%"}},
		BuildParams(group("and", cond("name", "like", "A%"))))
	require.Equal(t, []Param{{Key: "name", Value: "ilike.*son*"}},
		BuildParams(group("and", cond("name", "ilike", "*son*"))))
}

func TestBuildParams_MatchAndArrayOps(t *testing.T) {
	require.Equal(t, []Param{{Key: "name", Value: "match.^A.*z$"}},
		BuildParams(group("and", cond("name", "match", "^A.*z$"))))
	require.Equal(t, []Param{{Key: "tags", Value: "cs.{a,b}"}},
		BuildParams(group("and", cond("tags", "cs", "{a,b}"))))
}

func TestBuildParams_EmptyFieldSkipped(t *testing.T) {
	require.Nil(t, BuildParams(group("and", cond("", "eq", "x"))))
}

func TestBuildParams_BinaryMissingValueSkipped(t *testing.T) {
	require.Nil(t, BuildParams(group("and", cond("name", "eq", ""))))
}

func TestBuildParams_NestedOrInAnd(t *testing.T) {
	params := BuildParams(group("and",
		cond("status", "eq", "active"),
		group("or",
			cond("plan", "eq", "pro"),
			cond("plan", "eq", "enterprise"),
		),
	))
	require.Equal(t, []Param{
		{Key: "status", Value: "eq.active"},
		{Key: "or", Value: "(plan.eq.pro,plan.eq.enterprise)"},
	}, params)
}

func TestBuildParams_NestedAndInOr(t *testing.T) {
	// An AND group nested inside a top-level OR is serialised inline as and(...).
	params := BuildParams(group("or",
		cond("age", "eq", "14"),
		group("and",
			cond("age", "gte", "11"),
			cond("age", "lte", "17"),
		),
	))
	require.Equal(t, []Param{
		{Key: "or", Value: "(age.eq.14,and(age.gte.11,age.lte.17))"},
	}, params)
}

func TestBuildParams_NestedAndInAndFlattened(t *testing.T) {
	// An AND group nested in an AND group flattens into separate parameters.
	params := BuildParams(group("and",
		cond("a", "eq", "1"),
		group("and",
			cond("b", "eq", "2"),
			cond("c", "eq", "3"),
		),
	))
	require.Equal(t, []Param{
		{Key: "a", Value: "eq.1"},
		{Key: "b", Value: "eq.2"},
		{Key: "c", Value: "eq.3"},
	}, params)
}

func TestBuildParams_OrGroupQuotesReservedValue(t *testing.T) {
	// Inside a group, a value with a reserved character must be double-quoted so
	// the surrounding parentheses/commas are not mis-parsed.
	params := BuildParams(group("or",
		cond("name", "eq", "Smith, John"),
		cond("name", "eq", "Doe"),
	))
	require.Equal(t, []Param{
		{Key: "or", Value: `(name.eq."Smith, John",name.eq.Doe)`},
	}, params)
}

func TestBuildParams_SingleConditionOrGroup(t *testing.T) {
	params := BuildParams(group("or", cond("status", "eq", "active")))
	require.Equal(t, []Param{{Key: "or", Value: "(status.eq.active)"}}, params)
}

func TestBuildParams_EmptyNestedGroupSkipped(t *testing.T) {
	params := BuildParams(group("and",
		cond("a", "eq", "1"),
		group("or"),
	))
	require.Equal(t, []Param{{Key: "a", Value: "eq.1"}}, params)
}

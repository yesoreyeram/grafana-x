package plugin

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func TestBuildSearchQuery_DefaultWhenEmpty(t *testing.T) {
	require.Equal(t, defaultSearchQuery, buildSearchQuery(QueryModel{}))
}

func TestBuildSearchQuery_StructuredFilters(t *testing.T) {
	q := buildSearchQuery(QueryModel{
		StoryType:      "bug",
		Projects:       []string{"Backend"},
		WorkflowStates: []string{"In Progress"},
		Epic:           "Auth overhaul",
		Iteration:      "Sprint 12",
		Labels:         []string{"auth", "needs copy"},
		Owners:         []string{"alice"},
		Teams:          []string{"Platform Team"},
		Archived:       archivedOnly,
	})
	require.Contains(t, q, "type:bug")
	require.Contains(t, q, "project:Backend")
	require.Contains(t, q, `state:"In Progress"`)
	require.Contains(t, q, `epic:"Auth overhaul"`)
	require.Contains(t, q, `iteration:"Sprint 12"`)
	require.Contains(t, q, "label:auth")
	require.Contains(t, q, `label:"needs copy"`)
	require.Contains(t, q, "owner:alice")
	require.Contains(t, q, `team:"Platform Team"`)
	require.Contains(t, q, "is:archived")
	require.NotContains(t, q, "!is:archived")
}

func TestBuildSearchQuery_FreeTextPreserved(t *testing.T) {
	q := buildSearchQuery(QueryModel{Query: "login flow", StoryType: "feature"})
	require.Contains(t, q, "login flow")
	require.Contains(t, q, "type:feature")
}

func TestBuildSearchQuery_ArchivedExclude(t *testing.T) {
	q := buildSearchQuery(QueryModel{Archived: archivedExclude})
	require.Equal(t, "!is:archived", q)
}

func TestBuildSearchQuery_CustomDates(t *testing.T) {
	q := buildSearchQuery(QueryModel{
		DateMode:       dateModeCustom,
		CreatedAfter:   "2024-01-01",
		CreatedBefore:  "2024-12-31",
		UpdatedAfter:   "2024-06-01T10:00:00Z", // timestamp reduced to date part
		DeadlineBefore: "2025-01-01",
	})
	require.Contains(t, q, "created:2024-01-01..2024-12-31")
	require.Contains(t, q, "updated:2024-06-01..*")
	require.Contains(t, q, "due:*..2025-01-01")
}

func TestBuildSearchQuery_DashboardDateField(t *testing.T) {
	from := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 3, 31, 12, 0, 0, 0, time.UTC)
	tr := backend.TimeRange{From: from, To: to}

	q := buildSearchQuery(QueryModel{DateMode: dateModeDashboard, DateField: dateFieldCreated, TimeRange: tr})
	require.Contains(t, q, "created:2024-03-01..2024-03-31")

	q = buildSearchQuery(QueryModel{DateMode: dateModeDashboard, DateField: dateFieldUpdated, TimeRange: tr})
	require.Contains(t, q, "updated:2024-03-01..2024-03-31")

	q = buildSearchQuery(QueryModel{DateMode: dateModeDashboard, DateField: dateFieldDeadline, TimeRange: tr})
	require.Contains(t, q, "due:2024-03-01..2024-03-31")
}

func TestBuildSearchQuery_DashboardWithoutRangeIgnored(t *testing.T) {
	q := buildSearchQuery(QueryModel{DateMode: dateModeDashboard, DateField: dateFieldCreated})
	require.Equal(t, defaultSearchQuery, q)
}

func TestDateRange(t *testing.T) {
	require.Equal(t, "2024-01-01..2024-12-31", dateRange("2024-01-01", "2024-12-31"))
	require.Equal(t, "2024-01-01..*", dateRange("2024-01-01", ""))
	require.Equal(t, "*..2024-12-31", dateRange("", "2024-12-31"))
	require.Equal(t, "", dateRange("", ""))
	// Date terms pass through unchanged.
	require.Equal(t, "today..*", dateRange("today", ""))
}

func TestToSearchDate(t *testing.T) {
	require.Equal(t, "2024-01-01", toSearchDate("2024-01-01"))
	require.Equal(t, "2024-06-01", toSearchDate("2024-06-01T10:00:00Z"))
	require.Equal(t, "today", toSearchDate("today"))
	require.Equal(t, "", toSearchDate("  "))
}

func TestQuoteTerm(t *testing.T) {
	require.Equal(t, "bug", quoteTerm("bug"))
	require.Equal(t, `"In Progress"`, quoteTerm("In Progress"))
	require.Equal(t, `"In Progress"`, quoteTerm(`In "Progress"`)) // embedded quotes stripped
}

func TestNonEmpty(t *testing.T) {
	require.Equal(t, []string{"a", "b"}, nonEmpty([]string{"a", " ", "b", ""}))
	require.Empty(t, nonEmpty(nil))
}

package plugin

import "strings"

// Supported query types. Each maps to a Confluence content listing or to the
// CQL search endpoint.
const (
	// queryTypePages lists pages via GET /api/v2/pages.
	queryTypePages = "pages"
	// queryTypeBlogposts lists blog posts via GET /api/v2/blogposts.
	queryTypeBlogposts = "blogposts"
	// queryTypeSearch runs a CQL search via GET /rest/api/search.
	queryTypeSearch = "search"
	// queryTypeCount returns the number of matching items (pages by default, or
	// CQL search results when a CQL query is supplied).
	queryTypeCount = "count"
)

// listableQueryType reports whether the query type is one of the record-listing
// types handled by Client.ListRecords (everything except count).
func listableQueryType(qt string) bool {
	switch qt {
	case queryTypePages, queryTypeBlogposts, queryTypeSearch, "":
		return true
	default:
		return false
	}
}

// splitFields splits a comma-separated list of field names into a trimmed,
// non-empty slice.
func splitFields(fields string) []string {
	out := make([]string, 0)
	for _, f := range strings.Split(fields, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

package plugin

import "strings"

// Supported query types. Each maps to a HubSpot CRM object type or utility.
const (
	queryTypeContacts   = "contacts"
	queryTypeCompanies  = "companies"
	queryTypeDeals      = "deals"
	queryTypeTickets    = "tickets"
	queryTypeProducts   = "products"
	queryTypeLineItems  = "line_items"
	queryTypeMeetings   = "meetings"
	queryTypeCalls      = "calls"
	queryTypeTasks      = "tasks"
	queryTypeNotes      = "notes"
	queryTypeEmails     = "emails"
	queryTypePipelines  = "pipelines"
	queryTypeOwners     = "owners"
	queryTypeProperties = "properties"
	queryTypeRaw        = "raw"
)

// objectTypeToAPIPath maps query types (except pipelines/owners/properties/raw)
// to their HubSpot CRM Search API path segment.
var objectTypeToAPIPath = map[string]string{
	queryTypeContacts:  "contacts",
	queryTypeCompanies: "companies",
	queryTypeDeals:     "deals",
	queryTypeTickets:   "tickets",
	queryTypeProducts:  "products",
	queryTypeLineItems: "line_items",
	queryTypeMeetings:  "meetings",
	queryTypeCalls:     "calls",
	queryTypeTasks:     "tasks",
	queryTypeNotes:     "notes",
	queryTypeEmails:    "emails",
}

// searchableQueryTypes returns true if the query type uses the CRM Search API.
func searchableQueryType(qt string) bool {
	_, ok := objectTypeToAPIPath[qt]
	return ok
}

// HubSpot search operators supported by the CRM Search API.
var searchOperators = []string{
	"EQ",
	"NEQ",
	"GT",
	"GTE",
	"LT",
	"LTE",
	"BETWEEN",
	"IN",
	"NOT_IN",
	"HAS_PROPERTY",
	"NOT_HAS_PROPERTY",
	"CONTAINS_TOKEN",
	"NOT_CONTAINS_TOKEN",
	"STARTS_WITH",
	"STARTS_WITH_TOKEN",
}

// SearchOperators returns the supported CRM search operators.
func SearchOperators() []string {
	out := make([]string, len(searchOperators))
	copy(out, searchOperators)
	return out
}

// nonEmpty returns the trimmed, non-empty entries of a string slice.
func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

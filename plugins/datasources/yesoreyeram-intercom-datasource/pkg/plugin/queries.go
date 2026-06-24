package plugin

// Supported query types. Each maps to an Intercom entity or a utility.
const (
	queryTypeConversations = "conversations"
	queryTypeContacts      = "contacts"
	queryTypeTickets       = "tickets"
	queryTypeArticles      = "articles"
	queryTypeCompanies     = "companies"
	queryTypeAdmins        = "admins"
	queryTypeTeams         = "teams"
	queryTypeTags          = "tags"
	queryTypeCount         = "count"
)

// entityDataKey maps an entity to the JSON key under which its array of objects
// is returned by the Intercom API. Intercom is inconsistent here: list/search of
// contacts, articles, companies and tags return objects under `data`, while
// conversations/tickets/admins/teams use their own keys.
var entityDataKey = map[string]string{
	queryTypeConversations: "conversations",
	queryTypeContacts:      "data",
	queryTypeTickets:       "tickets",
	queryTypeArticles:      "data",
	queryTypeCompanies:     "data",
	queryTypeAdmins:        "admins",
	queryTypeTeams:         "teams",
	queryTypeTags:          "data",
}

// searchableEntity reports whether the entity exposes a POST /{entity}/search
// endpoint. Tickets are search-only (there is no GET /tickets list endpoint).
func searchableEntity(e string) bool {
	switch e {
	case queryTypeConversations, queryTypeContacts, queryTypeTickets:
		return true
	}
	return false
}

// cursorListEntity reports whether the entity exposes a GET list endpoint with
// cursor (or page) pagination.
func cursorListEntity(e string) bool {
	switch e {
	case queryTypeConversations, queryTypeContacts, queryTypeArticles, queryTypeCompanies:
		return true
	}
	return false
}

// simpleListEntity reports whether the entity exposes a single, non-paginated
// list endpoint (the response carries the full set in one call).
func simpleListEntity(e string) bool {
	switch e {
	case queryTypeAdmins, queryTypeTeams, queryTypeTags:
		return true
	}
	return false
}

// dataKeyFor returns the response array key for an entity, defaulting to `data`.
func dataKeyFor(e string) string {
	if k, ok := entityDataKey[e]; ok {
		return k
	}
	return "data"
}

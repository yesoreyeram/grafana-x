package plugin

// Supported query types. Each entity type maps to a Pipedrive v1 list endpoint;
// "count" returns the number of records matching the filters for a chosen
// entity.
const (
	queryTypeDeals         = "deals"
	queryTypePersons       = "persons"
	queryTypeOrganizations = "organizations"
	queryTypeProducts      = "products"
	queryTypeCount         = "count"
)

// entityListPath maps an entity query type to its Pipedrive v1 list path segment.
var entityListPath = map[string]string{
	queryTypeDeals:         "deals",
	queryTypePersons:       "persons",
	queryTypeOrganizations: "organizations",
	queryTypeProducts:      "products",
}

// entityFieldsPath maps an entity query type to the v1 field-definitions endpoint
// used to translate 40-character custom field hash keys into human-readable
// names (DealField.key -> DealField.name).
var entityFieldsPath = map[string]string{
	queryTypeDeals:         "dealFields",
	queryTypePersons:       "personFields",
	queryTypeOrganizations: "organizationFields",
	queryTypeProducts:      "productFields",
}

// isEntityQuery reports whether qt is a list-able entity (deals/persons/etc).
func isEntityQuery(qt string) bool {
	_, ok := entityListPath[qt]
	return ok
}

// ----- Standard field catalogs -----------------------------------------------
//
// These document the common standard (non-custom) fields returned by each
// Pipedrive entity. They are not used to restrict the response (Pipedrive
// returns every field plus custom-field hashes); they serve as a reference for
// the standard schema and for field-name hints in the editor.

// DealFieldNames are common standard Pipedrive deal fields.
var DealFieldNames = []string{
	"id", "title", "value", "currency", "status", "stage_id", "pipeline_id",
	"user_id", "person_id", "org_id", "add_time", "update_time", "close_time",
	"won_time", "lost_time", "expected_close_date", "probability", "lost_reason",
	"visible_to", "active", "deleted", "stage_change_time", "next_activity_date",
	"last_activity_date", "owner_name", "stage_order_nr",
}

// PersonFieldNames are common standard Pipedrive person fields.
var PersonFieldNames = []string{
	"id", "name", "first_name", "last_name", "email", "phone", "owner_id",
	"org_id", "add_time", "update_time", "open_deals_count", "won_deals_count",
	"last_activity_date", "next_activity_date", "active_flag",
}

// OrganizationFieldNames are common standard Pipedrive organization fields.
var OrganizationFieldNames = []string{
	"id", "name", "owner_id", "add_time", "update_time", "address",
	"people_count", "open_deals_count", "won_deals_count", "active_flag",
}

// ProductFieldNames are common standard Pipedrive product fields.
var ProductFieldNames = []string{
	"id", "name", "code", "unit", "tax", "category", "active_flag",
	"selectable", "prices", "add_time", "update_time", "owner_id",
}

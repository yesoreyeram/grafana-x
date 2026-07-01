package plugin

import "strings"

// systemFields lists the column names that the QueryEditor's "Hide system
// fields" toggle removes from the returned frame for this data source. They
// are the metadata-style columns that come back from the API but are typically
// not part of the user's domain data.
var systemFields = map[string]bool{
	"id":              true,
	"added_at":        true,
	"added_by_uid":    true,
	"assigned_by_uid": true,
	"updated_at":      true,
	"completed_at":    true,
	"user_id":         true,
	"project_id":      true,
	"section_id":      true,
	"parent_id":       true,
	"child_order":     true,
	"creator_id":      true,
	"day_order":       true,
	"sync_id":         true,
	"is_collapsed":    true,
	"is_deleted":      true,
	"v2_id":           true,
	"v2_parent_id":    true,
	"v2_project_id":   true,
	"v2_section_id":   true,
}

// dropSystemFields removes from each record the columns listed in systemFields
// plus any column whose name starts with an underscore (the conventional
// "internal" prefix used by several upstream APIs). It mutates and returns the
// slice for convenience.
func dropSystemFields(records []map[string]any) []map[string]any {
	for _, rec := range records {
		for k := range rec {
			if systemFields[k] || strings.HasPrefix(k, "_") {
				delete(rec, k)
			}
		}
	}
	return records
}

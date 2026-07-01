package plugin

import "strings"

// systemFields lists the column names that the QueryEditor's "Hide system
// fields" toggle removes from the returned frame for this data source. They
// are the metadata-style columns that come back from the API but are typically
// not part of the user's domain data.
var systemFields = map[string]bool{
	"id":                 true,
	"add_time":           true,
	"update_time":        true,
	"stage_change_time":  true,
	"won_time":           true,
	"lost_time":          true,
	"close_time":         true,
	"last_activity_date": true,
	"next_activity_date": true,
	"active_flag":        true,
	"deleted":            true,
	"visible_to":         true,
	"owner_id":           true,
	"creator_user_id":    true,
	"user_id":            true,
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

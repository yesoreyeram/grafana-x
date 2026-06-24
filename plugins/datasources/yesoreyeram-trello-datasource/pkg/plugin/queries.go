package plugin

import "sort"

// cardFieldNames is the catalog of selectable card columns (after flattening).
// These mirror the scalar columns produced by flattenCard in frame.go, so the
// names match the output columns rather than the raw API shape. Selecting a
// subset restricts the returned columns; an empty selection returns them all.
var cardFieldNames = []string{
	"id",
	"name",
	"desc",
	"closed",
	"pos",
	"shortUrl",
	"url",
	"idList",
	"idBoard",
	"idMembers",
	"labels",
	"idChecklists",
	"due",
	"dueComplete",
	"start",
	"dateCreated",
	"dateLastActivity",
	"badges_votes",
	"badges_comments",
	"badges_attachments",
	"badges_checkItems",
	"badges_checkItemsChecked",
	"customFieldItems",
}

// CardFieldNames returns the sorted catalog of selectable card field names, used
// to populate the fields multi-select in the query editor.
func CardFieldNames() []string {
	names := make([]string, len(cardFieldNames))
	copy(names, cardFieldNames)
	sort.Strings(names)
	return names
}

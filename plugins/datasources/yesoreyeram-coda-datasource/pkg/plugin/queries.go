package plugin

// Supported query types.
const (
	QueryTypeRows  = "rows"
	QueryTypeCount = "count"
)

// validSortBy is the set of values accepted by Coda's rows `sortBy` parameter
// (the RowsSortBy enum). Anything else is ignored when building requests.
var validSortBy = map[string]bool{
	"createdAt": true,
	"updatedAt": true,
	"natural":   true,
}

// validValueFormat is the set of values accepted by Coda's rows `valueFormat`
// parameter.
var validValueFormat = map[string]bool{
	"simple":           true,
	"simpleWithArrays": true,
	"rich":             true,
}

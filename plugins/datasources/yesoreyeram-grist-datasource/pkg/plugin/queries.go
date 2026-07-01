package plugin

// Query types supported by the data source.
const (
	// QueryTypeRecords lists records from a table (the default).
	QueryTypeRecords = "records"
	// QueryTypeCount returns the number of matching records via SQL COUNT(*).
	QueryTypeCount = "count"
	// QueryTypeSQL runs a raw read-only Grist SQL SELECT statement.
	QueryTypeSQL = "sql"
)

package plugin

// Query types supported by the data source.
const (
	// QueryTypeRecords lists rows from a table (the default).
	QueryTypeRecords = "records"
	// QueryTypeCount returns the number of matching rows via SQL COUNT(*).
	QueryTypeCount = "count"
	// QueryTypeSQL runs a raw SeaTable SQL statement.
	QueryTypeSQL = "sql"
)

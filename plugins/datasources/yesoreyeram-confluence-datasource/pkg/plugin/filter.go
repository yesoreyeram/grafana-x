package plugin

import "strings"

// Confluence does not use a structured JSON filter object (unlike Notion or
// Directus). Filtering is expressed either as query parameters (space-id on the
// pages/blogposts endpoints) or, for the search endpoint, as a free-form CQL
// string. This file centralises the small amount of CQL string handling the
// backend performs so it stays in one well-tested place.

// BuildCQL normalises a user-supplied CQL string before it is sent to the search
// endpoint. It only trims surrounding whitespace — the CQL grammar itself is the
// user's responsibility and is validated server-side by Confluence.
func BuildCQL(cql string) string {
	return strings.TrimSpace(cql)
}

// EscapeCQLValue escapes a raw value so it can be safely embedded inside a CQL
// string literal (double-quoted). Backslashes and double quotes are escaped per
// the CQL syntax rules.
func EscapeCQLValue(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return v
}

// SpaceCQL builds a CQL clause that scopes a search to a single space key, e.g.
// `space = "ENG"`. It returns an empty string when no space key is provided.
func SpaceCQL(spaceKey string) string {
	spaceKey = strings.TrimSpace(spaceKey)
	if spaceKey == "" {
		return ""
	}
	return `space = "` + EscapeCQLValue(spaceKey) + `"`
}

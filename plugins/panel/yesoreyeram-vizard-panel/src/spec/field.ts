/**
 * Escape a Grafana field/column name for use as a Vega-Lite *field reference*.
 *
 * Vega-Lite interprets `.` and `[`/`]` in a field string as nested-property /
 * array access. Grafana display names can contain those characters (e.g.
 * "response.time"), so we escape them with a backslash to force a literal match
 * against the row-object key. Names with only spaces or braces (e.g. label-based
 * names like "value {host=a}") are returned unchanged.
 */
export function escapeFieldName(name: string): string {
  return name
    .replace(/\\/g, '\\\\')
    .replace(/\./g, '\\.')
    .replace(/\[/g, '\\[')
    .replace(/\]/g, '\\]');
}

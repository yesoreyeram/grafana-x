import { SelectableValue } from '@grafana/data';

// ---------------------------------------------------------------------------
// Search filter model
//
// A SearchFilter mirrors a single Intercom Search API condition
// `{field, operator, value}`. The backend (pkg/plugin/filter.go::BuildSearchQuery)
// combines the structured pickers and these generic rows into the Search API
// `query` object. Multiple rows are AND'd server-side.
// ---------------------------------------------------------------------------

export interface SearchFilter {
  /** Intercom search field (e.g. role, state, created_at, tag_ids). */
  field: string;
  /** Intercom search operator (see INTERCOM_OPERATORS). */
  operator: string;
  /** Comparison value. Numeric/timestamp fields are coerced to numbers server-side. */
  value: string;
}

export function newFilter(): SearchFilter {
  return { field: '', operator: '=', value: '' };
}

// ---------------------------------------------------------------------------
// Operator catalog
//
// The Intercom Search API supports a fixed set of operators. `~`/`!~` are
// substring (contains) matches; `^`/`$` are starts-with / ends-with; `IN`/`NIN`
// expect a comma-separated value list.
// ---------------------------------------------------------------------------

export interface OperatorDef {
  value: string;
  label: string;
}

export const INTERCOM_OPERATORS: OperatorDef[] = [
  { value: '=', label: '= (equals)' },
  { value: '!=', label: '!= (not equals)' },
  { value: '>', label: '> (greater than)' },
  { value: '<', label: '< (less than)' },
  { value: '~', label: '~ (contains)' },
  { value: '!~', label: '!~ (not contains)' },
  { value: '^', label: '^ (starts with)' },
  { value: '$', label: '$ (ends with)' },
  { value: 'IN', label: 'IN (any of)' },
  { value: 'NIN', label: 'NIN (none of)' },
];

export function operatorOptions(): Array<SelectableValue<string>> {
  return INTERCOM_OPERATORS.map((o) => ({ label: o.label, value: o.value }));
}

/** Returns true for operators that take a comma-separated list of values. */
export function isListOperator(op: string): boolean {
  return op === 'IN' || op === 'NIN';
}

// ---------------------------------------------------------------------------
// Template interpolation
//
// Backend data sources interpolate the query on the frontend before it is sent,
// so each filter value is interpolated here (arity aware). List operators use
// `csv` formatting so a multi-value variable expands to comma-separated tokens.
// ---------------------------------------------------------------------------

export type ValueInterpolator = (value: string, asList: boolean) => string;

export function interpolateFilters(filters: SearchFilter[] | undefined, interpolate: ValueInterpolator): SearchFilter[] | undefined {
  if (!filters) {
    return filters;
  }
  return filters.map((f) => {
    if (!f.value) {
      return f;
    }
    return { ...f, value: interpolate(f.value, isListOperator(f.operator)) };
  });
}

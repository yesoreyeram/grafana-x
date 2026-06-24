import { SelectableValue } from '@grafana/data';

// ---------------------------------------------------------------------------
// Filter model
//
// The structured filter tree is compiled into PostgREST query parameters on the
// backend. The frontend only describes conditions/groups; it does not build the
// query parameters itself.
// ---------------------------------------------------------------------------

export type LogicalConnector = 'and' | 'or';

export interface FilterCondition {
  kind: 'condition';
  /** Column name. */
  field: string;
  /** Operator (see OPERATORS). */
  op: string;
  /** Comparison value. Ignored for unary operators. */
  value?: string;
}

export interface FilterGroup {
  kind: 'group';
  /** How the direct children are combined (AND/OR). */
  connector: LogicalConnector;
  children: FilterNode[];
}

export type FilterNode = FilterCondition | FilterGroup;

export function newCondition(): FilterCondition {
  return { kind: 'condition', field: '', op: 'eq', value: '' };
}

export function newGroup(): FilterGroup {
  return { kind: 'group', connector: 'and', children: [] };
}

export function emptyRootGroup(): FilterGroup {
  return newGroup();
}

// ---------------------------------------------------------------------------
// Operator catalog
// ---------------------------------------------------------------------------

export interface OperatorDef {
  value: string;
  label: string;
  /**
   * 'none'   = unary (no value)
   * 'single' = one value
   */
  arity: 'none' | 'single';
}

// Operator tokens map directly onto PostgREST operators. A `not.` prefix negates
// the operator; the `is` family carries its target inline (is.null / is.true /
// …). The backend (filter.go) compiles these into query parameters.
const EQUALITY: OperatorDef[] = [
  { value: 'eq', label: '=', arity: 'single' },
  { value: 'neq', label: '!=', arity: 'single' },
];

const COMPARISON: OperatorDef[] = [
  { value: 'gt', label: '>', arity: 'single' },
  { value: 'gte', label: '>=', arity: 'single' },
  { value: 'lt', label: '<', arity: 'single' },
  { value: 'lte', label: '<=', arity: 'single' },
];

const TEXT_MATCH: OperatorDef[] = [
  { value: 'like', label: 'like (use * or %)', arity: 'single' },
  { value: 'ilike', label: 'ilike (case-insensitive)', arity: 'single' },
  { value: 'match', label: 'matches regex (~)', arity: 'single' },
  { value: 'imatch', label: 'matches regex, case-insensitive (~*)', arity: 'single' },
];

const LIST: OperatorDef[] = [
  { value: 'in', label: 'in (comma-separated list)', arity: 'single' },
];

const ARRAY: OperatorDef[] = [
  { value: 'cs', label: 'contains (@>)', arity: 'single' },
  { value: 'cd', label: 'contained in (<@)', arity: 'single' },
];

const NULLABLE: OperatorDef[] = [
  { value: 'is.null', label: 'is null', arity: 'none' },
  { value: 'not.is.null', label: 'is not null', arity: 'none' },
  { value: 'is.true', label: 'is true', arity: 'none' },
  { value: 'is.false', label: 'is false', arity: 'none' },
];

const ALL_OPERATORS: OperatorDef[] = [
  ...EQUALITY,
  ...COMPARISON,
  ...TEXT_MATCH,
  ...LIST,
  ...ARRAY,
  ...NULLABLE,
];

export function operatorsForType(_type?: string): OperatorDef[] {
  return ALL_OPERATORS;
}

export function operatorOptions(type?: string): Array<SelectableValue<string>> {
  return operatorsForType(type).map((o) => ({ label: o.label, value: o.value }));
}

export function operatorArity(op: string): 'none' | 'single' {
  return ALL_OPERATORS.find((o) => o.value === op)?.arity ?? 'single';
}

// ---------------------------------------------------------------------------
// Template interpolation
// ---------------------------------------------------------------------------

export type ValueInterpolator = (value: string) => string;

export function interpolateFilterTree(root: FilterGroup, interpolate: ValueInterpolator): FilterGroup {
  const mapNode = (node: FilterNode): FilterNode => {
    if (node.kind === 'group') {
      return { ...node, children: node.children.map(mapNode) };
    }
    if (!node.value) {
      return node;
    }
    return { ...node, value: interpolate(node.value) };
  };
  return { ...root, children: root.children.map(mapNode) };
}

// ---------------------------------------------------------------------------
// JSON (de)serialization for persistence on the query
// ---------------------------------------------------------------------------

export function parseFilterTree(raw?: string): FilterGroup {
  if (!raw) {
    return emptyRootGroup();
  }
  try {
    const parsed = JSON.parse(raw);
    if (parsed && parsed.kind === 'group' && Array.isArray(parsed.children)) {
      return parsed as FilterGroup;
    }
  } catch {
    // ignore malformed persisted state
  }
  return emptyRootGroup();
}

export function stringifyFilterTree(root: FilterGroup): string {
  if (root.children.length === 0) {
    return '';
  }
  return JSON.stringify(root);
}

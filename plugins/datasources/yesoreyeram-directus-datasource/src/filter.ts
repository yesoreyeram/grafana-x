import { SelectableValue } from '@grafana/data';

// ---------------------------------------------------------------------------
// Filter model
//
// The structured filter tree is compiled into a Directus JSON filter object on
// the backend. The frontend only describes conditions/groups; it does not build
// the filter itself.
// ---------------------------------------------------------------------------

export type LogicalConnector = 'and' | 'or';

export interface FilterCondition {
  kind: 'condition';
  /** Directus field name. */
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
// Operator catalog (field-type aware)
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

const EQUALITY: OperatorDef[] = [
  { value: 'eq', label: '=', arity: 'single' },
  { value: 'neq', label: '!=', arity: 'single' },
];

const NULLABLE: OperatorDef[] = [
  { value: 'empty', label: 'is empty', arity: 'none' },
  { value: 'not_empty', label: 'is not empty', arity: 'none' },
];

const COMPARISON: OperatorDef[] = [
  { value: 'gt', label: '>', arity: 'single' },
  { value: 'gte', label: '>=', arity: 'single' },
  { value: 'lt', label: '<', arity: 'single' },
  { value: 'lte', label: '<=', arity: 'single' },
];

const TEXT_MATCH: OperatorDef[] = [
  { value: 'contains', label: 'contains', arity: 'single' },
  { value: 'ncontains', label: 'does not contain', arity: 'single' },
  { value: 'startsWith', label: 'starts with', arity: 'single' },
  { value: 'endsWith', label: 'ends with', arity: 'single' },
];

const LIST_MATCH: OperatorDef[] = [
  { value: 'in', label: 'is one of (csv)', arity: 'single' },
  { value: 'nin', label: 'is not one of (csv)', arity: 'single' },
];

const RANGE: OperatorDef[] = [
  { value: 'between', label: 'is between (min,max)', arity: 'single' },
  { value: 'nbetween', label: 'is not between (min,max)', arity: 'single' },
];

/** Logical categories a Directus field type can map to for operator selection. */
export type FieldCategory = 'text' | 'number' | 'boolean' | 'date' | 'json';

const TYPE_CATEGORY: Record<string, FieldCategory> = {
  string: 'text',
  text: 'text',
  varchar: 'text',
  char: 'text',
  tinytext: 'text',
  mediumtext: 'text',
  longtext: 'text',
  json: 'json',
  alias: 'text',
  integer: 'number',
  bigInteger: 'number',
  smallInteger: 'number',
  decimal: 'number',
  float: 'number',
  double: 'number',
  boolean: 'boolean',
  date: 'date',
  dateTime: 'date',
  timestamp: 'date',
  time: 'date',
};

export function categoryForType(type?: string): FieldCategory {
  if (!type) {
    return 'text';
  }
  return TYPE_CATEGORY[type] ?? 'text';
}

export function operatorsForType(type?: string): OperatorDef[] {
  switch (categoryForType(type)) {
    case 'number':
      return [...EQUALITY, ...COMPARISON, ...RANGE, ...LIST_MATCH, ...NULLABLE];
    case 'boolean':
      return [...EQUALITY, ...NULLABLE];
    case 'date':
      return [...EQUALITY, ...COMPARISON, ...RANGE, ...NULLABLE];
    case 'json':
      return [...EQUALITY, ...NULLABLE];
    case 'text':
    default:
      return [...EQUALITY, ...TEXT_MATCH, ...LIST_MATCH, ...NULLABLE];
  }
}

export function operatorOptions(type?: string): Array<SelectableValue<string>> {
  return operatorsForType(type).map((o) => ({ label: o.label, value: o.value }));
}

const ALL_OPERATORS: OperatorDef[] = [
  ...EQUALITY,
  ...NULLABLE,
  ...COMPARISON,
  ...TEXT_MATCH,
  ...LIST_MATCH,
  ...RANGE,
];

export function operatorArity(op: string): 'none' | 'single' {
  return ALL_OPERATORS.find((o) => o.value === op)?.arity ?? 'single';
}

/**
 * List-style operators take a comma-separated value that the backend compiles
 * into a JSON array (_in/_nin) or a [min,max] range (_between/_nbetween). Their
 * template-variable values are interpolated with `csv` formatting so a
 * multi-value variable expands to comma-separated tokens.
 */
const LIST_OPERATORS = new Set(['in', 'nin', 'between', 'nbetween']);

export function isListOperator(op: string): boolean {
  return LIST_OPERATORS.has(op);
}

// ---------------------------------------------------------------------------
// Template interpolation
// ---------------------------------------------------------------------------

export type ValueInterpolator = (value: string, asList: boolean) => string;

export function interpolateFilterTree(root: FilterGroup, interpolate: ValueInterpolator): FilterGroup {
  const mapNode = (node: FilterNode): FilterNode => {
    if (node.kind === 'group') {
      return { ...node, children: node.children.map(mapNode) };
    }
    if (!node.value) {
      return node;
    }
    return { ...node, value: interpolate(node.value, isListOperator(node.op)) };
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

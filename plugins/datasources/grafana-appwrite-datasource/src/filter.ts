import { SelectableValue } from '@grafana/data';

// ---------------------------------------------------------------------------
// Filter model
//
// The structured filter tree is compiled into Appwrite query strings on the
// backend. The frontend only describes conditions/groups; it does not build the
// query strings itself.
// ---------------------------------------------------------------------------

export type LogicalConnector = 'and' | 'or';

export interface FilterCondition {
  kind: 'condition';
  /** Appwrite attribute key. */
  attribute: string;
  /** Operator (see OPERATORS). Matches the Appwrite query method names. */
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
  return { kind: 'condition', attribute: '', op: 'equal', value: '' };
}

export function newGroup(): FilterGroup {
  return { kind: 'group', connector: 'and', children: [] };
}

export function emptyRootGroup(): FilterGroup {
  return newGroup();
}

// ---------------------------------------------------------------------------
// Operator catalog (attribute-type aware)
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
  { value: 'equal', label: '=', arity: 'single' },
  { value: 'notEqual', label: '!=', arity: 'single' },
];

const NULLABLE: OperatorDef[] = [
  { value: 'isNull', label: 'is null', arity: 'none' },
  { value: 'isNotNull', label: 'is not null', arity: 'none' },
];

const COMPARISON: OperatorDef[] = [
  { value: 'greaterThan', label: '>', arity: 'single' },
  { value: 'greaterThanEqual', label: '>=', arity: 'single' },
  { value: 'lessThan', label: '<', arity: 'single' },
  { value: 'lessThanEqual', label: '<=', arity: 'single' },
];

const TEXT_MATCH: OperatorDef[] = [
  { value: 'contains', label: 'contains', arity: 'single' },
  { value: 'notContains', label: 'does not contain', arity: 'single' },
  { value: 'startsWith', label: 'starts with', arity: 'single' },
  { value: 'endsWith', label: 'ends with', arity: 'single' },
  { value: 'search', label: 'search (full-text)', arity: 'single' },
];

/** Logical categories an Appwrite attribute type can map to for operator selection. */
export type AttributeCategory = 'text' | 'number' | 'boolean' | 'datetime';

// Map Appwrite attribute types to a logical category. Unknown types default to text.
const TYPE_CATEGORY: Record<string, AttributeCategory> = {
  // text
  string: 'text',
  email: 'text',
  url: 'text',
  ip: 'text',
  enum: 'text',
  relationship: 'text',
  // number
  integer: 'number',
  double: 'number',
  // boolean
  boolean: 'boolean',
  // date / time
  datetime: 'datetime',
};

export function categoryForType(type?: string): AttributeCategory {
  if (!type) {
    return 'text';
  }
  return TYPE_CATEGORY[type] ?? 'text';
}

/** Returns the operator definitions valid for a given Appwrite attribute type. */
export function operatorsForType(type?: string): OperatorDef[] {
  switch (categoryForType(type)) {
    case 'number':
      return [...EQUALITY, ...COMPARISON, ...NULLABLE];
    case 'boolean':
      return [...EQUALITY, ...NULLABLE];
    case 'datetime':
      return [...EQUALITY, ...COMPARISON, ...NULLABLE];
    case 'text':
    default:
      return [...EQUALITY, ...TEXT_MATCH, ...NULLABLE];
  }
}

export function operatorOptions(type?: string): Array<SelectableValue<string>> {
  return operatorsForType(type).map((o) => ({ label: o.label, value: o.value }));
}

const ALL_OPERATORS: OperatorDef[] = [...EQUALITY, ...NULLABLE, ...COMPARISON, ...TEXT_MATCH];

export function operatorArity(op: string): 'none' | 'single' {
  return ALL_OPERATORS.find((o) => o.value === op)?.arity ?? 'single';
}

// ---------------------------------------------------------------------------
// Template interpolation
//
// The query strings are built on the backend from the structured tree. Because
// backend data sources interpolate the query on the frontend before it is sent,
// we interpolate each condition value here so the backend receives concrete
// values.
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
  // Empty tree persists as empty string to keep queries clean.
  if (root.children.length === 0) {
    return '';
  }
  return JSON.stringify(root);
}

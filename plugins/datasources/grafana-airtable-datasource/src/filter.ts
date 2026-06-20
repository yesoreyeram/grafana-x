import { SelectableValue } from '@grafana/data';

// ---------------------------------------------------------------------------
// Filter model
//
// The structured filter tree is compiled into an Airtable `filterByFormula`
// expression on the backend. The frontend only describes conditions/groups; it
// does not build the formula itself.
// ---------------------------------------------------------------------------

export type LogicalConnector = 'and' | 'or';

export interface FilterCondition {
  kind: 'condition';
  /** Airtable field name. */
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
  { value: 'not_contains', label: 'does not contain', arity: 'single' },
];

const BOOL: OperatorDef[] = [
  { value: 'is_true', label: 'is checked', arity: 'none' },
  { value: 'is_false', label: 'is unchecked', arity: 'none' },
];

/** Logical categories an Airtable field type can map to for operator selection. */
export type FieldCategory = 'text' | 'number' | 'boolean' | 'date';

// Map Airtable field types to a logical category. Unknown types default to text.
const TYPE_CATEGORY: Record<string, FieldCategory> = {
  // text
  singleLineText: 'text',
  multilineText: 'text',
  richText: 'text',
  email: 'text',
  url: 'text',
  phoneNumber: 'text',
  singleSelect: 'text',
  multipleSelects: 'text',
  multipleRecordLinks: 'text',
  multipleCollaborators: 'text',
  singleCollaborator: 'text',
  barcode: 'text',
  // number
  number: 'number',
  percent: 'number',
  currency: 'number',
  rating: 'number',
  duration: 'number',
  count: 'number',
  autoNumber: 'number',
  // boolean
  checkbox: 'boolean',
  // date / time
  date: 'date',
  dateTime: 'date',
  createdTime: 'date',
  lastModifiedTime: 'date',
};

export function categoryForType(type?: string): FieldCategory {
  if (!type) {
    return 'text';
  }
  return TYPE_CATEGORY[type] ?? 'text';
}

/** Returns the operator definitions valid for a given Airtable field type. */
export function operatorsForType(type?: string): OperatorDef[] {
  switch (categoryForType(type)) {
    case 'number':
      return [...EQUALITY, ...COMPARISON, ...NULLABLE];
    case 'boolean':
      return [...BOOL];
    case 'date':
      return [...EQUALITY, ...COMPARISON, ...NULLABLE];
    case 'text':
    default:
      return [...EQUALITY, ...TEXT_MATCH, ...NULLABLE];
  }
}

export function operatorOptions(type?: string): Array<SelectableValue<string>> {
  return operatorsForType(type).map((o) => ({ label: o.label, value: o.value }));
}

const ALL_OPERATORS: OperatorDef[] = [...EQUALITY, ...NULLABLE, ...COMPARISON, ...TEXT_MATCH, ...BOOL];

export function operatorArity(op: string): 'none' | 'single' {
  return ALL_OPERATORS.find((o) => o.value === op)?.arity ?? 'single';
}

// ---------------------------------------------------------------------------
// Template interpolation
//
// The formula is built on the backend from the structured tree. Because backend
// data sources interpolate the query on the frontend before it is sent, we
// interpolate each condition value here so the backend receives concrete values.
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

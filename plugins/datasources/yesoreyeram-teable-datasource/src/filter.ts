import { SelectableValue } from '@grafana/data';

// ---------------------------------------------------------------------------
// Filter model
//
// The structured filter tree is compiled into a Teable JSON `filter` object
// ({conjunction, filterSet:[...]}) on the backend. The frontend only describes
// conditions/groups; it does not build the filter object itself. Operators are
// Teable's native operators and field references use field NAMES (the backend
// queries Teable with fieldKeyType=name).
// ---------------------------------------------------------------------------

export type LogicalConnector = 'and' | 'or';

/** Logical categories a Teable field type can map to for operator selection. */
export type FieldCategory = 'text' | 'number' | 'boolean' | 'date' | 'select' | 'multiSelect' | 'attachment';

export interface FilterCondition {
  kind: 'condition';
  /** Teable field name. */
  field: string;
  /** Logical category of the field; drives backend value coercion/shaping. */
  category?: FieldCategory;
  /** Teable operator (see OPERATORS). */
  op: string;
  /** Comparison value. Ignored for unary operators; comma-separated for lists. */
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
  return { kind: 'condition', field: '', category: 'text', op: 'is', value: '' };
}

export function newGroup(): FilterGroup {
  return { kind: 'group', connector: 'and', children: [] };
}

export function emptyRootGroup(): FilterGroup {
  return newGroup();
}

// ---------------------------------------------------------------------------
// Operator catalog (field-type aware) — Teable native operators
// ---------------------------------------------------------------------------

export interface OperatorDef {
  value: string;
  label: string;
  /**
   * 'none'   = unary (no value)
   * 'single' = one value
   * 'list'   = comma-separated values compiled into an array
   */
  arity: 'none' | 'single' | 'list';
}

const TEXT: OperatorDef[] = [
  { value: 'is', label: 'is', arity: 'single' },
  { value: 'isNot', label: 'is not', arity: 'single' },
  { value: 'contains', label: 'contains', arity: 'single' },
  { value: 'doesNotContain', label: 'does not contain', arity: 'single' },
];

const NUMBER: OperatorDef[] = [
  { value: 'is', label: '=', arity: 'single' },
  { value: 'isNot', label: '!=', arity: 'single' },
  { value: 'isGreater', label: '>', arity: 'single' },
  { value: 'isGreaterEqual', label: '>=', arity: 'single' },
  { value: 'isLess', label: '<', arity: 'single' },
  { value: 'isLessEqual', label: '<=', arity: 'single' },
];

const DATE: OperatorDef[] = [
  { value: 'is', label: 'is', arity: 'single' },
  { value: 'isNot', label: 'is not', arity: 'single' },
  { value: 'isBefore', label: 'is before', arity: 'single' },
  { value: 'isAfter', label: 'is after', arity: 'single' },
  { value: 'isOnOrBefore', label: 'is on or before', arity: 'single' },
  { value: 'isOnOrAfter', label: 'is on or after', arity: 'single' },
];

const SELECT: OperatorDef[] = [
  { value: 'is', label: 'is', arity: 'single' },
  { value: 'isNot', label: 'is not', arity: 'single' },
  { value: 'isAnyOf', label: 'is any of', arity: 'list' },
  { value: 'isNoneOf', label: 'is none of', arity: 'list' },
];

const MULTI_SELECT: OperatorDef[] = [
  { value: 'hasAnyOf', label: 'has any of', arity: 'list' },
  { value: 'hasAllOf', label: 'has all of', arity: 'list' },
  { value: 'hasNoneOf', label: 'has none of', arity: 'list' },
  { value: 'isExactly', label: 'is exactly', arity: 'list' },
];

const BOOL: OperatorDef[] = [{ value: 'is', label: 'is', arity: 'single' }];

const NULLABLE: OperatorDef[] = [
  { value: 'isEmpty', label: 'is empty', arity: 'none' },
  { value: 'isNotEmpty', label: 'is not empty', arity: 'none' },
];

// Map Teable field types to a logical category. Unknown types default to text.
const TYPE_CATEGORY: Record<string, FieldCategory> = {
  // text
  singleLineText: 'text',
  longText: 'text',
  formula: 'text',
  rollup: 'text',
  conditionalRollup: 'text',
  button: 'text',
  // number
  number: 'number',
  rating: 'number',
  autoNumber: 'number',
  // boolean
  checkbox: 'boolean',
  // date / time
  date: 'date',
  createdTime: 'date',
  lastModifiedTime: 'date',
  // single-value reference / choice
  singleSelect: 'select',
  user: 'select',
  link: 'select',
  createdBy: 'select',
  lastModifiedBy: 'select',
  // multi-value
  multipleSelect: 'multiSelect',
  // attachment supports emptiness checks only
  attachment: 'attachment',
};

export function categoryForType(type?: string): FieldCategory {
  if (!type) {
    return 'text';
  }
  return TYPE_CATEGORY[type] ?? 'text';
}

/** Returns the operator definitions valid for a given Teable field type. */
export function operatorsForType(type?: string): OperatorDef[] {
  switch (categoryForType(type)) {
    case 'number':
      return [...NUMBER, ...NULLABLE];
    case 'boolean':
      return [...BOOL];
    case 'date':
      return [...DATE, ...NULLABLE];
    case 'select':
      return [...SELECT, ...NULLABLE];
    case 'multiSelect':
      return [...MULTI_SELECT, ...NULLABLE];
    case 'attachment':
      return [...NULLABLE];
    case 'text':
    default:
      return [...TEXT, ...NULLABLE];
  }
}

export function operatorOptions(type?: string): Array<SelectableValue<string>> {
  return operatorsForType(type).map((o) => ({ label: o.label, value: o.value }));
}

const ALL_OPERATORS: OperatorDef[] = [
  ...TEXT,
  ...NUMBER,
  ...DATE,
  ...SELECT,
  ...MULTI_SELECT,
  ...NULLABLE,
];

export function operatorArity(op: string): 'none' | 'single' | 'list' {
  return ALL_OPERATORS.find((o) => o.value === op)?.arity ?? 'single';
}

// ---------------------------------------------------------------------------
// Template interpolation
//
// The filter object is built on the backend from the structured tree. Because
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

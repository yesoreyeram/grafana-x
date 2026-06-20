import { SelectableValue } from '@grafana/data';

// ---------------------------------------------------------------------------
// Filter model
// ---------------------------------------------------------------------------

export type LogicalConnector = 'and' | 'or';

export interface FilterCondition {
  kind: 'condition';
  /** Notion property name. */
  field: string;
  /** Logical property category used by the backend to pick the Notion filter type key. */
  category?: FieldCategory;
  /** Comparison operator (Notion operator name; see OPERATORS). */
  op: string;
  /** Comparison value. Ignored for unary operators; comma-separated for lists. */
  value?: string;
}

export interface FilterGroup {
  kind: 'group';
  /** How the direct children are combined. */
  connector: LogicalConnector;
  children: FilterNode[];
}

export type FilterNode = FilterCondition | FilterGroup;

export function newCondition(): FilterCondition {
  return { kind: 'condition', field: '', category: 'text', op: 'equals', value: '' };
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
   * 'none'  = unary (no value)
   * 'single'= one value
   * 'list'  = comma-separated list (expanded into an or/and group server-side)
   */
  arity: 'none' | 'single' | 'list';
}

// Operator groups reused across field categories. The `value` is the exact
// Notion API operator name.
const EQUALITY: OperatorDef[] = [
  { value: 'equals', label: '=', arity: 'single' },
  { value: 'does_not_equal', label: '!=', arity: 'single' },
];

const NULLABLE: OperatorDef[] = [
  { value: 'is_empty', label: 'is empty', arity: 'none' },
  { value: 'is_not_empty', label: 'is not empty', arity: 'none' },
];

const NUMBER_COMPARISON: OperatorDef[] = [
  { value: 'greater_than', label: '>', arity: 'single' },
  { value: 'greater_than_or_equal_to', label: '>=', arity: 'single' },
  { value: 'less_than', label: '<', arity: 'single' },
  { value: 'less_than_or_equal_to', label: '<=', arity: 'single' },
];

const DATE_COMPARISON: OperatorDef[] = [
  { value: 'before', label: 'is before', arity: 'single' },
  { value: 'after', label: 'is after', arity: 'single' },
  { value: 'on_or_before', label: 'is on or before', arity: 'single' },
  { value: 'on_or_after', label: 'is on or after', arity: 'single' },
];

const TEXT_MATCH: OperatorDef[] = [
  { value: 'contains', label: 'contains', arity: 'single' },
  { value: 'does_not_contain', label: 'does not contain', arity: 'single' },
  { value: 'starts_with', label: 'starts with', arity: 'single' },
  { value: 'ends_with', label: 'ends with', arity: 'single' },
];

const SELECT_LIST: OperatorDef[] = [
  { value: 'in', label: 'is any of', arity: 'list' },
  { value: 'not_in', label: 'is none of', arity: 'list' },
];

const MULTI_OPS: OperatorDef[] = [
  { value: 'contains', label: 'contains', arity: 'single' },
  { value: 'does_not_contain', label: 'does not contain', arity: 'single' },
  { value: 'in', label: 'has any of', arity: 'list' },
  { value: 'not_in', label: 'has none of', arity: 'list' },
];

const BOOL: OperatorDef[] = [{ value: 'equals', label: '=', arity: 'single' }];

/** Logical categories a Notion property type can map to for operator selection. */
export type FieldCategory = 'text' | 'number' | 'checkbox' | 'date' | 'select' | 'status' | 'multi_select' | 'people' | 'files';

// Map raw Notion property types to a logical category.
const TYPE_CATEGORY: Record<string, FieldCategory> = {
  // text
  title: 'text',
  rich_text: 'text',
  email: 'text',
  phone_number: 'text',
  url: 'text',
  formula: 'text',
  rollup: 'text',
  relation: 'text',
  // number
  number: 'number',
  unique_id: 'number',
  // checkbox
  checkbox: 'checkbox',
  // date / time
  date: 'date',
  created_time: 'date',
  last_edited_time: 'date',
  // single select
  select: 'select',
  status: 'status',
  // multi / arrays
  multi_select: 'multi_select',
  people: 'people',
  created_by: 'people',
  last_edited_by: 'people',
  files: 'files',
};

export function categoryForType(type?: string): FieldCategory {
  if (!type) {
    return 'text';
  }
  return TYPE_CATEGORY[type] ?? 'text';
}

/** Returns the operator definitions valid for a given Notion property type. */
export function operatorsForType(type?: string): OperatorDef[] {
  switch (categoryForType(type)) {
    case 'number':
      return [...EQUALITY, ...NUMBER_COMPARISON, ...NULLABLE];
    case 'checkbox':
      return [...BOOL];
    case 'date':
      return [...EQUALITY, ...DATE_COMPARISON, ...NULLABLE];
    case 'select':
    case 'status':
      return [...EQUALITY, ...SELECT_LIST, ...NULLABLE];
    case 'multi_select':
    case 'people':
      return [...MULTI_OPS, ...NULLABLE];
    case 'files':
      return [...NULLABLE];
    case 'text':
    default:
      return [...EQUALITY, ...TEXT_MATCH, ...NULLABLE];
  }
}

export function operatorOptions(type?: string): Array<SelectableValue<string>> {
  return operatorsForType(type).map((o) => ({ label: o.label, value: o.value }));
}

export function operatorDef(op: string, type?: string): OperatorDef | undefined {
  return operatorsForType(type).find((o) => o.value === op) ?? ALL_OPERATORS.find((o) => o.value === op);
}

const ALL_OPERATORS: OperatorDef[] = [
  ...EQUALITY,
  ...NULLABLE,
  ...NUMBER_COMPARISON,
  ...DATE_COMPARISON,
  ...TEXT_MATCH,
  ...SELECT_LIST,
  { value: 'contains', label: 'contains', arity: 'single' },
  { value: 'does_not_contain', label: 'does not contain', arity: 'single' },
];

export function operatorArity(op: string): 'none' | 'single' | 'list' {
  return ALL_OPERATORS.find((o) => o.value === op)?.arity ?? 'single';
}

// ---------------------------------------------------------------------------
// Template interpolation
//
// The Notion filter object is built on the backend from the structured tree.
// Because backend data sources interpolate the query on the frontend before it
// is sent, we interpolate each condition value here (arity aware), so the
// backend receives concrete values.
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
    const asList = operatorArity(node.op) === 'list';
    return { ...node, value: interpolate(node.value, asList) };
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

import { SelectableValue } from '@grafana/data';

// ---------------------------------------------------------------------------
// Filter model
// ---------------------------------------------------------------------------

export type LogicalConnector = 'and' | 'or';

export interface FilterCondition {
  kind: 'condition';
  /** Baserow field name. */
  field: string;
  /** Baserow filter type (see OPERATORS). */
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
  return { kind: 'condition', field: '', op: 'equal', value: '' };
}

export function newGroup(): FilterGroup {
  return { kind: 'group', connector: 'and', children: [] };
}

export function emptyRootGroup(): FilterGroup {
  return newGroup();
}

// ---------------------------------------------------------------------------
// Operator catalog (field-type aware)
//
// Operators are Baserow view filter types. See the `filters` parameter on
// GET /api/database/rows/table/{table_id}/ in the Baserow API docs.
// ---------------------------------------------------------------------------

export interface OperatorDef {
  value: string;
  label: string;
  /**
   * 'none'   = unary (no value)
   * 'single' = one value
   * 'list'   = comma-separated list of values
   */
  arity: 'none' | 'single' | 'list';
}

// Operator groups reused across field categories.
const EQUALITY: OperatorDef[] = [
  { value: 'equal', label: '=', arity: 'single' },
  { value: 'not_equal', label: '!=', arity: 'single' },
];

const NULLABLE: OperatorDef[] = [
  { value: 'empty', label: 'is empty', arity: 'none' },
  { value: 'not_empty', label: 'is not empty', arity: 'none' },
];

const COMPARISON: OperatorDef[] = [
  { value: 'higher_than', label: '>', arity: 'single' },
  { value: 'higher_than_or_equal', label: '>=', arity: 'single' },
  { value: 'lower_than', label: '<', arity: 'single' },
  { value: 'lower_than_or_equal', label: '<=', arity: 'single' },
];

const TEXT_MATCH: OperatorDef[] = [
  { value: 'contains', label: 'contains', arity: 'single' },
  { value: 'contains_not', label: 'does not contain', arity: 'single' },
  { value: 'contains_word', label: 'contains word', arity: 'single' },
  { value: 'doesnt_contain_word', label: 'does not contain word', arity: 'single' },
];

const BOOL: OperatorDef[] = [{ value: 'boolean', label: 'is', arity: 'single' }];

const SINGLE_SELECT_OPS: OperatorDef[] = [
  { value: 'single_select_equal', label: 'is', arity: 'single' },
  { value: 'single_select_not_equal', label: 'is not', arity: 'single' },
  { value: 'single_select_is_any_of', label: 'is any of', arity: 'list' },
  { value: 'single_select_is_none_of', label: 'is none of', arity: 'list' },
];

const MULTI_SELECT_OPS: OperatorDef[] = [
  { value: 'multiple_select_has', label: 'has', arity: 'single' },
  { value: 'multiple_select_has_not', label: 'has not', arity: 'single' },
];

const LINK_ROW_OPS: OperatorDef[] = [
  { value: 'link_row_has', label: 'has', arity: 'single' },
  { value: 'link_row_has_not', label: 'has not', arity: 'single' },
  { value: 'link_row_contains', label: 'contains', arity: 'single' },
  { value: 'link_row_not_contains', label: 'does not contain', arity: 'single' },
];

const DATE_OPS: OperatorDef[] = [
  { value: 'date_is', label: 'is', arity: 'single' },
  { value: 'date_is_not', label: 'is not', arity: 'single' },
  { value: 'date_is_before', label: 'is before', arity: 'single' },
  { value: 'date_is_on_or_before', label: 'is on or before', arity: 'single' },
  { value: 'date_is_after', label: 'is after', arity: 'single' },
  { value: 'date_is_on_or_after', label: 'is on or after', arity: 'single' },
];

/** Logical categories a Baserow field type can map to for operator selection. */
export type FieldCategory = 'text' | 'number' | 'boolean' | 'date' | 'single_select' | 'multi_select' | 'link_row';

// Map Baserow field types to a logical category.
const TYPE_CATEGORY: Record<string, FieldCategory> = {
  // text
  text: 'text',
  long_text: 'text',
  url: 'text',
  email: 'text',
  phone_number: 'text',
  uuid: 'text',
  password: 'text',
  // number
  number: 'number',
  rating: 'number',
  duration: 'number',
  count: 'number',
  rollup: 'number',
  autonumber: 'number',
  // boolean
  boolean: 'boolean',
  // date / time
  date: 'date',
  created_on: 'date',
  last_modified: 'date',
  // single select
  single_select: 'single_select',
  // multi select / arrays
  multiple_select: 'multi_select',
  multiple_collaborators: 'multi_select',
  // link rows
  link_row: 'link_row',
};

export function categoryForType(type?: string): FieldCategory {
  if (!type) {
    return 'text';
  }
  return TYPE_CATEGORY[type] ?? 'text';
}

/** Returns the operator definitions valid for a given Baserow field type. */
export function operatorsForType(type?: string): OperatorDef[] {
  switch (categoryForType(type)) {
    case 'number':
      return [...EQUALITY, ...COMPARISON, ...NULLABLE];
    case 'boolean':
      return [...BOOL];
    case 'date':
      return [...DATE_OPS, ...NULLABLE];
    case 'single_select':
      return [...SINGLE_SELECT_OPS, ...NULLABLE];
    case 'multi_select':
      return [...MULTI_SELECT_OPS, ...NULLABLE];
    case 'link_row':
      return [...LINK_ROW_OPS, ...NULLABLE];
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
  ...COMPARISON,
  ...TEXT_MATCH,
  ...BOOL,
  ...SINGLE_SELECT_OPS,
  ...MULTI_SELECT_OPS,
  ...LINK_ROW_OPS,
  ...DATE_OPS,
];

export function operatorArity(op: string): 'none' | 'single' | 'list' {
  return ALL_OPERATORS.find((o) => o.value === op)?.arity ?? 'single';
}

// ---------------------------------------------------------------------------
// Template interpolation
//
// The Baserow `filters` clause is built on the backend from the structured
// tree. Because backend data sources interpolate the query on the frontend
// before it is sent, we interpolate each condition value here (arity aware), so
// the backend receives concrete values.
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

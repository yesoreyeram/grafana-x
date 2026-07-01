import { SelectableValue } from '@grafana/data';

// ---------------------------------------------------------------------------
// Filter model
//
// The structured filter tree is compiled into a Grist `filter` JSON object
// on the backend. The frontend only describes conditions/groups; it does not
// build the filter JSON itself.
// ---------------------------------------------------------------------------

export type LogicalConnector = 'and' | 'or';

export interface FilterCondition {
  kind: 'condition';
  /** Grist column name. */
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

export type OperatorArity = 'none' | 'single' | 'list';

export interface OperatorDef {
  value: string;
  label: string;
  /**
   * 'none'   = unary (no value)
   * 'single' = one value
   * 'list'   = comma-separated values
   */
  arity: OperatorArity;
}

const EQUALITY: OperatorDef[] = [
  { value: 'eq', label: '=', arity: 'single' },
  { value: 'neq', label: '!=', arity: 'single' },
];

const MEMBERSHIP: OperatorDef[] = [
  { value: 'in', label: 'in (any of)', arity: 'list' },
  { value: 'not_in', label: 'not in', arity: 'list' },
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

/** Logical categories a Grist field type can map to for operator selection. */
export type FieldCategory = 'text' | 'number' | 'boolean' | 'date';

const TYPE_CATEGORY: Record<string, FieldCategory> = {
  Text: 'text',
  Numeric: 'number',
  Integer: 'number',
  Decimal: 'number',
  Bool: 'boolean',
  Date: 'date',
  DateTime: 'date',
  Choice: 'text',
  ChoiceList: 'text',
  Ref: 'text',
  RefList: 'text',
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
      return [...EQUALITY, ...COMPARISON, ...MEMBERSHIP, ...NULLABLE];
    case 'boolean':
      return [...EQUALITY, ...NULLABLE];
    case 'date':
      return [...EQUALITY, ...COMPARISON, ...NULLABLE];
    case 'text':
    default:
      return [...EQUALITY, ...TEXT_MATCH, ...MEMBERSHIP, ...NULLABLE];
  }
}

export function operatorOptions(type?: string): Array<SelectableValue<string>> {
  return operatorsForType(type).map((o) => ({ label: o.label, value: o.value }));
}

const ALL_OPERATORS: OperatorDef[] = [...EQUALITY, ...MEMBERSHIP, ...NULLABLE, ...COMPARISON, ...TEXT_MATCH];

export function operatorArity(op: string): OperatorArity {
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

import { SelectableValue } from '@grafana/data';

// ---------------------------------------------------------------------------
// Filter model
//
// The structured filter tree is compiled into an Attio JSON filter object on the
// backend. The frontend only describes conditions/groups; it does not build the
// filter itself. Each condition carries the attribute's logical `category` so
// the backend can coerce the value to the right JSON type.
// ---------------------------------------------------------------------------

export type LogicalConnector = 'and' | 'or';

/** Logical categories an Attio attribute type maps to for operator selection. */
export type FieldCategory = 'text' | 'number' | 'boolean' | 'date';

export interface FilterCondition {
  kind: 'condition';
  /** Attio attribute slug. */
  field: string;
  /** Logical category of the attribute (drives value coercion server-side). */
  category?: FieldCategory;
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
  return { kind: 'condition', field: '', category: 'text', op: 'eq', value: '' };
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
  { value: 'startsWith', label: 'starts with', arity: 'single' },
  { value: 'endsWith', label: 'ends with', arity: 'single' },
];

const LIST_MATCH: OperatorDef[] = [{ value: 'in', label: 'is one of', arity: 'single' }];

// Maps an Attio attribute type to the logical category used for operator
// selection and value coercion.
const TYPE_CATEGORY: Record<string, FieldCategory> = {
  number: 'number',
  currency: 'number',
  rating: 'number',
  checkbox: 'boolean',
  date: 'date',
  timestamp: 'date',
  text: 'text',
  status: 'text',
  select: 'text',
  'record-reference': 'text',
  'actor-reference': 'text',
  'email-address': 'text',
  'phone-number': 'text',
  domain: 'text',
  location: 'text',
  'personal-name': 'text',
  interaction: 'text',
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
      return [...EQUALITY, ...COMPARISON, ...NULLABLE];
    case 'boolean':
      return [...EQUALITY];
    case 'date':
      return [...EQUALITY, ...COMPARISON, ...NULLABLE];
    case 'text':
    default:
      return [...EQUALITY, ...TEXT_MATCH, ...LIST_MATCH, ...NULLABLE];
  }
}

export function operatorOptions(type?: string): Array<SelectableValue<string>> {
  return operatorsForType(type).map((o) => ({ label: o.label, value: o.value }));
}

const ALL_OPERATORS: OperatorDef[] = [...EQUALITY, ...NULLABLE, ...COMPARISON, ...TEXT_MATCH, ...LIST_MATCH];

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

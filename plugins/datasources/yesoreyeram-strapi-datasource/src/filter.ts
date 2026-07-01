import { SelectableValue } from '@grafana/data';

export type LogicalConnector = 'and' | 'or';

export interface FilterCondition {
  kind: 'condition';
  /** Strapi field name (from attributes). */
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

// Operator catalog (field-type aware)

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
  { value: 'ne', label: '!=', arity: 'single' },
];

const NULLABLE: OperatorDef[] = [
  { value: 'null', label: 'is null', arity: 'none' },
  { value: 'notNull', label: 'is not null', arity: 'none' },
];

const COMPARISON: OperatorDef[] = [
  { value: 'gt', label: '>', arity: 'single' },
  { value: 'gte', label: '>=', arity: 'single' },
  { value: 'lt', label: '<', arity: 'single' },
  { value: 'lte', label: '<=', arity: 'single' },
];

const TEXT_MATCH: OperatorDef[] = [
  { value: 'contains', label: 'contains', arity: 'single' },
  { value: 'containsi', label: 'contains (case-insensitive)', arity: 'single' },
  { value: 'notContains', label: 'does not contain', arity: 'single' },
  { value: 'startsWith', label: 'starts with', arity: 'single' },
  { value: 'endsWith', label: 'ends with', arity: 'single' },
];

const LIST_MATCH: OperatorDef[] = [
  { value: 'in', label: 'is one of', arity: 'single' },
  { value: 'notIn', label: 'is not one of', arity: 'single' },
];

/** Logical categories a Strapi field type can map to for operator selection. */
export type FieldCategory = 'text' | 'number' | 'boolean' | 'date' | 'json';

const TYPE_CATEGORY: Record<string, FieldCategory> = {
  string: 'text',
  text: 'text',
  email: 'text',
  password: 'text',
  uid: 'text',
  enumeration: 'text',
  richtext: 'text',
  integer: 'number',
  biginteger: 'number',
  float: 'number',
  decimal: 'number',
  number: 'number',
  boolean: 'boolean',
  date: 'date',
  time: 'date',
  datetime: 'date',
  timestamp: 'date',
  json: 'json',
  media: 'text',
  relation: 'text',
  component: 'json',
  dynamiczone: 'json',
};

export function categoryForType(type?: string): FieldCategory {
  if (!type) {
    return 'text';
  }
  return TYPE_CATEGORY[type.toLowerCase()] ?? 'text';
}

export function operatorsForType(type?: string): OperatorDef[] {
  switch (categoryForType(type)) {
    case 'number':
      return [...EQUALITY, ...COMPARISON, ...NULLABLE];
    case 'boolean':
      return [...EQUALITY, ...NULLABLE];
    case 'date':
      return [...EQUALITY, ...COMPARISON, ...NULLABLE];
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

const ALL_OPERATORS: OperatorDef[] = [...EQUALITY, ...NULLABLE, ...COMPARISON, ...TEXT_MATCH, ...LIST_MATCH];

export function operatorArity(op: string): 'none' | 'single' {
  return ALL_OPERATORS.find((o) => o.value === op)?.arity ?? 'single';
}

// Template interpolation

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

// JSON (de)serialization for persistence on the query

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

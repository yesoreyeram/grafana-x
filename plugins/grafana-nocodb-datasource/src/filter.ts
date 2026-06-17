import { SelectableValue } from '@grafana/data';

// ---------------------------------------------------------------------------
// Filter model
// ---------------------------------------------------------------------------

export type LogicalConnector = 'and' | 'or';

export interface FilterCondition {
  kind: 'condition';
  /** NocoDB column/field title. */
  field: string;
  /** Comparison operator (see OPERATORS). */
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
   * 'none'  = unary (no value)
   * 'single'= one quoted value
   * 'list'  = comma-separated list of values (NocoDB expects unquoted tokens)
   */
  arity: 'none' | 'single' | 'list';
}

// Operator groups reused across field categories.
const EQUALITY: OperatorDef[] = [
  { value: 'eq', label: '=', arity: 'single' },
  { value: 'neq', label: '!=', arity: 'single' },
];

const NULLABLE: OperatorDef[] = [
  { value: 'blank', label: 'is blank', arity: 'none' },
  { value: 'notblank', label: 'is not blank', arity: 'none' },
];

const COMPARISON: OperatorDef[] = [
  { value: 'gt', label: '>', arity: 'single' },
  { value: 'ge', label: '>=', arity: 'single' },
  { value: 'lt', label: '<', arity: 'single' },
  { value: 'le', label: '<=', arity: 'single' },
];

const TEXT_MATCH: OperatorDef[] = [
  { value: 'like', label: 'is like', arity: 'single' },
  { value: 'nlike', label: 'is not like', arity: 'single' },
];

const LIST_MEMBERSHIP: OperatorDef[] = [
  { value: 'in', label: 'is any of', arity: 'list' },
];

const BOOL: OperatorDef[] = [{ value: 'eq', label: '=', arity: 'single' }];

const ARRAY_OPS: OperatorDef[] = [
  { value: 'anyof', label: 'has any of', arity: 'list' },
  { value: 'allof', label: 'has all of', arity: 'list' },
  { value: 'nanyof', label: 'has none of', arity: 'list' },
  { value: 'nallof', label: 'has not all of', arity: 'list' },
];

const DATE_OPS: OperatorDef[] = [
  ...EQUALITY,
  ...COMPARISON,
  { value: 'isWithin', label: 'is within', arity: 'single' },
  ...NULLABLE,
];

/** Logical categories a NocoDB uidt can map to for operator selection. */
export type FieldCategory = 'text' | 'number' | 'boolean' | 'date' | 'array' | 'select';

// Map NocoDB UI data types (uidt) to a logical category.
const UIDT_CATEGORY: Record<string, FieldCategory> = {
  // text
  SingleLineText: 'text',
  LongText: 'text',
  Email: 'text',
  PhoneNumber: 'text',
  URL: 'text',
  RichText: 'text',
  JSON: 'text',
  // number
  Number: 'number',
  Decimal: 'number',
  Currency: 'number',
  Percent: 'number',
  Rating: 'number',
  Duration: 'number',
  Year: 'number',
  AutoNumber: 'number',
  // boolean
  Checkbox: 'boolean',
  // date / time
  Date: 'date',
  DateTime: 'date',
  Time: 'date',
  CreatedTime: 'date',
  LastModifiedTime: 'date',
  // single select
  SingleSelect: 'select',
  // array / multi
  MultiSelect: 'array',
  LinkToAnotherRecord: 'array',
  Links: 'array',
  User: 'array',
};

export function categoryForType(uidt?: string): FieldCategory {
  if (!uidt) {
    return 'text';
  }
  return UIDT_CATEGORY[uidt] ?? 'text';
}

/** Returns the operator definitions valid for a given NocoDB field type. */
export function operatorsForType(uidt?: string): OperatorDef[] {
  switch (categoryForType(uidt)) {
    case 'number':
      return [...EQUALITY, ...COMPARISON, ...NULLABLE];
    case 'boolean':
      return [...BOOL];
    case 'date':
      return [...DATE_OPS];
    case 'select':
      return [...EQUALITY, ...LIST_MEMBERSHIP, ...NULLABLE];
    case 'array':
      return [...ARRAY_OPS, ...NULLABLE];
    case 'text':
    default:
      return [...EQUALITY, ...TEXT_MATCH, ...LIST_MEMBERSHIP, ...NULLABLE];
  }
}

export function operatorOptions(uidt?: string): Array<SelectableValue<string>> {
  return operatorsForType(uidt).map((o) => ({ label: o.label, value: o.value }));
}

export function operatorDef(op: string, uidt?: string): OperatorDef | undefined {
  return operatorsForType(uidt).find((o) => o.value === op) ?? ALL_OPERATORS.find((o) => o.value === op);
}

const ALL_OPERATORS: OperatorDef[] = [
  ...EQUALITY,
  ...NULLABLE,
  ...COMPARISON,
  ...TEXT_MATCH,
  ...LIST_MEMBERSHIP,
  ...ARRAY_OPS,
  { value: 'isWithin', label: 'is within', arity: 'single' },
];

export function operatorArity(op: string): 'none' | 'single' | 'list' {
  return ALL_OPERATORS.find((o) => o.value === op)?.arity ?? 'single';
}

// ---------------------------------------------------------------------------
// Serialization to NocoDB where syntax
// ---------------------------------------------------------------------------

// NocoDB v2 `where` grammar (when prefixed with @, values may be quoted):
//   condition: (field,op[,value[,value2]])
//   group:     (s1)~and(s2)~or(s3)   (siblings joined by the group connector)
// We use the v3-style quoting via the `@` prefix so values containing commas or
// parentheses are handled safely.

// ---------------------------------------------------------------------------
// Template interpolation
//
// The NocoDB where clause is built on the backend from the structured tree.
// Because backend data sources interpolate the query on the frontend before it
// is sent, we interpolate each condition value here (type/arity aware), so the
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

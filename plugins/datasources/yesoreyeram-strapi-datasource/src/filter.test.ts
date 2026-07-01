import {
  categoryForType,
  emptyRootGroup,
  interpolateFilterTree,
  newCondition,
  operatorArity,
  operatorsForType,
  parseFilterTree,
  stringifyFilterTree,
  FilterGroup,
} from './filter';

describe('operator catalog', () => {
  it('maps Strapi field types to categories', () => {
    expect(categoryForType('integer')).toBe('number');
    expect(categoryForType('biginteger')).toBe('number');
    expect(categoryForType('decimal')).toBe('number');
    expect(categoryForType('string')).toBe('text');
    expect(categoryForType('text')).toBe('text');
    expect(categoryForType('enumeration')).toBe('text');
    expect(categoryForType('boolean')).toBe('boolean');
    expect(categoryForType('datetime')).toBe('date');
    expect(categoryForType('date')).toBe('date');
    expect(categoryForType('json')).toBe('json');
    // case-insensitive lookup (Strapi schema types vary in casing)
    expect(categoryForType('DateTime')).toBe('date');
    expect(categoryForType(undefined)).toBe('text');
    expect(categoryForType('unknown_type')).toBe('text');
  });

  it('text fields get contains, case-insensitive contains and list operators', () => {
    const ops = operatorsForType('string').map((o) => o.value);
    expect(ops).toEqual(
      expect.arrayContaining(['eq', 'ne', 'contains', 'containsi', 'notContains', 'startsWith', 'endsWith', 'in', 'notIn', 'null', 'notNull'])
    );
  });

  it('number fields get comparison, not contains', () => {
    const ops = operatorsForType('integer').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['eq', 'ne', 'gt', 'gte', 'lt', 'lte']));
    expect(ops).not.toContain('contains');
  });

  it('reports operator arity', () => {
    expect(operatorArity('eq')).toBe('single');
    expect(operatorArity('null')).toBe('none');
    expect(operatorArity('notNull')).toBe('none');
    expect(operatorArity('contains')).toBe('single');
    expect(operatorArity('unknown')).toBe('single');
  });
});

describe('filter tree persistence', () => {
  it('parses empty/invalid to an empty root group', () => {
    expect(parseFilterTree()).toEqual(emptyRootGroup());
    expect(parseFilterTree('')).toEqual(emptyRootGroup());
    expect(parseFilterTree('not-json')).toEqual(emptyRootGroup());
  });

  it('serializes an empty tree to an empty string', () => {
    expect(stringifyFilterTree(emptyRootGroup())).toBe('');
  });

  it('round-trips a non-empty tree', () => {
    const tree: FilterGroup = {
      kind: 'group',
      connector: 'and',
      children: [{ kind: 'condition', field: 'status', op: 'eq', value: 'published' }],
    };
    expect(parseFilterTree(stringifyFilterTree(tree))).toEqual(tree);
  });
});

describe('interpolation', () => {
  it('interpolates condition values, leaving structure intact', () => {
    const tree: FilterGroup = {
      kind: 'group',
      connector: 'and',
      children: [
        { kind: 'condition', field: 'status', op: 'eq', value: '$status' },
        {
          kind: 'group',
          connector: 'or',
          children: [{ kind: 'condition', field: 'author', op: 'eq', value: '$author' }],
        },
      ],
    };
    const out = interpolateFilterTree(tree, (v) => v.replace('$status', 'published').replace('$author', 'alice'));
    expect((out.children[0] as any).value).toBe('published');
    expect(((out.children[1] as FilterGroup).children[0] as any).value).toBe('alice');
  });

  it('leaves valueless conditions untouched', () => {
    const c = newCondition();
    const tree: FilterGroup = { kind: 'group', connector: 'and', children: [{ ...c, value: '' }] };
    const out = interpolateFilterTree(tree, () => 'SHOULD_NOT_APPEAR');
    expect((out.children[0] as any).value).toBe('');
  });
});

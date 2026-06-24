import {
  categoryForType,
  emptyRootGroup,
  interpolateFilterTree,
  isListOperator,
  newCondition,
  operatorArity,
  operatorsForType,
  parseFilterTree,
  stringifyFilterTree,
  FilterGroup,
} from './filter';

describe('operator catalog', () => {
  it('maps Directus field types to categories', () => {
    expect(categoryForType('integer')).toBe('number');
    expect(categoryForType('float')).toBe('number');
    expect(categoryForType('string')).toBe('text');
    expect(categoryForType('text')).toBe('text');
    expect(categoryForType('boolean')).toBe('boolean');
    expect(categoryForType('dateTime')).toBe('date');
    expect(categoryForType('timestamp')).toBe('date');
    expect(categoryForType(undefined)).toBe('text');
    expect(categoryForType('unknown_type')).toBe('text');
  });

  it('text fields get contains, starts/ends with and list operators', () => {
    const ops = operatorsForType('string').map((o) => o.value);
    expect(ops).toEqual(
      expect.arrayContaining([
        'eq',
        'neq',
        'contains',
        'ncontains',
        'startsWith',
        'endsWith',
        'in',
        'nin',
        'empty',
        'not_empty',
      ])
    );
  });

  it('number fields get comparison and between, not contains', () => {
    const ops = operatorsForType('integer').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['eq', 'neq', 'gt', 'gte', 'lt', 'lte', 'between', 'nbetween']));
    expect(ops).not.toContain('contains');
  });

  it('date fields get comparison and between', () => {
    const ops = operatorsForType('dateTime').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['eq', 'neq', 'gt', 'lt', 'between', 'nbetween']));
  });

  it('reports operator arity', () => {
    expect(operatorArity('eq')).toBe('single');
    expect(operatorArity('empty')).toBe('none');
    expect(operatorArity('contains')).toBe('single');
    expect(operatorArity('unknown')).toBe('single');
  });

  it('identifies list-style operators', () => {
    expect(isListOperator('in')).toBe(true);
    expect(isListOperator('nin')).toBe(true);
    expect(isListOperator('between')).toBe(true);
    expect(isListOperator('nbetween')).toBe(true);
    expect(isListOperator('eq')).toBe(false);
    expect(isListOperator('contains')).toBe(false);
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

  it('flags list operators so the caller can apply csv formatting', () => {
    const tree: FilterGroup = {
      kind: 'group',
      connector: 'and',
      children: [
        { kind: 'condition', field: 'status', op: 'in', value: '$statuses' },
        { kind: 'condition', field: 'title', op: 'eq', value: '$title' },
      ],
    };
    const asListFlags: boolean[] = [];
    interpolateFilterTree(tree, (value, asList) => {
      asListFlags.push(asList);
      return value;
    });
    expect(asListFlags).toEqual([true, false]);
  });
});

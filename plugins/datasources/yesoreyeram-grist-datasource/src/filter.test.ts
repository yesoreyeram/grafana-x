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
  it('maps Grist field types to categories', () => {
    expect(categoryForType('Text')).toBe('text');
    expect(categoryForType('Numeric')).toBe('number');
    expect(categoryForType('Integer')).toBe('number');
    expect(categoryForType('Bool')).toBe('boolean');
    expect(categoryForType('DateTime')).toBe('date');
    expect(categoryForType(undefined)).toBe('text');
    expect(categoryForType('UnknownType')).toBe('text');
  });

  it('text fields get contains operators but not numeric comparison', () => {
    const ops = operatorsForType('Text').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['eq', 'neq', 'contains', 'not_contains', 'empty', 'not_empty']));
    expect(ops).not.toContain('gt');
  });

  it('number fields get comparison and membership, not contains', () => {
    const ops = operatorsForType('Numeric').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['eq', 'neq', 'gt', 'gte', 'lt', 'lte', 'in', 'not_in']));
    expect(ops).not.toContain('contains');
  });

  it('text fields get membership operators', () => {
    const ops = operatorsForType('Text').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['in', 'not_in']));
  });

  it('reports operator arity', () => {
    expect(operatorArity('eq')).toBe('single');
    expect(operatorArity('empty')).toBe('none');
    expect(operatorArity('contains')).toBe('single');
    expect(operatorArity('in')).toBe('list');
    expect(operatorArity('not_in')).toBe('list');
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
      children: [{ kind: 'condition', field: 'Plan', op: 'eq', value: 'pro' }],
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
        { kind: 'condition', field: 'Plan', op: 'eq', value: '$plan' },
        {
          kind: 'group',
          connector: 'or',
          children: [{ kind: 'condition', field: 'Owner', op: 'eq', value: '$user' }],
        },
      ],
    };
    const out = interpolateFilterTree(tree, (v) => v.replace('$plan', 'pro').replace('$user', 'alice'));
    expect((out.children[0] as any).value).toBe('pro');
    expect(((out.children[1] as FilterGroup).children[0] as any).value).toBe('alice');
  });

  it('leaves valueless conditions untouched', () => {
    const c = newCondition();
    const tree: FilterGroup = { kind: 'group', connector: 'and', children: [{ ...c, value: '' }] };
    const out = interpolateFilterTree(tree, () => 'SHOULD_NOT_APPEAR');
    expect((out.children[0] as any).value).toBe('');
  });
});

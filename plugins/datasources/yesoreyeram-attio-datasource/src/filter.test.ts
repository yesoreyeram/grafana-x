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
  it('maps Attio attribute types to categories', () => {
    expect(categoryForType('number')).toBe('number');
    expect(categoryForType('currency')).toBe('number');
    expect(categoryForType('rating')).toBe('number');
    expect(categoryForType('checkbox')).toBe('boolean');
    expect(categoryForType('date')).toBe('date');
    expect(categoryForType('timestamp')).toBe('date');
    expect(categoryForType('text')).toBe('text');
    expect(categoryForType('status')).toBe('text');
    expect(categoryForType('select')).toBe('text');
    expect(categoryForType('record-reference')).toBe('text');
    expect(categoryForType(undefined)).toBe('text');
    expect(categoryForType('unknown_type')).toBe('text');
  });

  it('text fields get contains and list operators', () => {
    const ops = operatorsForType('text').map((o) => o.value);
    expect(ops).toEqual(
      expect.arrayContaining(['eq', 'neq', 'contains', 'startsWith', 'endsWith', 'in', 'empty', 'not_empty'])
    );
  });

  it('number fields get comparison, not contains', () => {
    const ops = operatorsForType('number').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['eq', 'neq', 'gt', 'gte', 'lt', 'lte']));
    expect(ops).not.toContain('contains');
  });

  it('boolean fields get only equality operators', () => {
    const ops = operatorsForType('checkbox').map((o) => o.value);
    expect(ops).toEqual(['eq', 'neq']);
  });

  it('reports operator arity', () => {
    expect(operatorArity('eq')).toBe('single');
    expect(operatorArity('empty')).toBe('none');
    expect(operatorArity('not_empty')).toBe('none');
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
      children: [{ kind: 'condition', field: 'stage', category: 'text', op: 'eq', value: 'Won' }],
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
        { kind: 'condition', field: 'stage', category: 'text', op: 'eq', value: '$stage' },
        {
          kind: 'group',
          connector: 'or',
          children: [{ kind: 'condition', field: 'owner', category: 'text', op: 'eq', value: '$owner' }],
        },
      ],
    };
    const out = interpolateFilterTree(tree, (v) => v.replace('$stage', 'Won').replace('$owner', 'alice'));
    expect((out.children[0] as any).value).toBe('Won');
    expect(((out.children[1] as FilterGroup).children[0] as any).value).toBe('alice');
  });

  it('leaves valueless conditions untouched', () => {
    const c = newCondition();
    const tree: FilterGroup = { kind: 'group', connector: 'and', children: [{ ...c, value: '' }] };
    const out = interpolateFilterTree(tree, () => 'SHOULD_NOT_APPEAR');
    expect((out.children[0] as any).value).toBe('');
  });
});

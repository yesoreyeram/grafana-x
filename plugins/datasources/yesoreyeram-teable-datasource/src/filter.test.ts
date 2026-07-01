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
  it('maps Teable field types to categories', () => {
    expect(categoryForType('number')).toBe('number');
    expect(categoryForType('rating')).toBe('number');
    expect(categoryForType('autoNumber')).toBe('number');
    expect(categoryForType('singleLineText')).toBe('text');
    expect(categoryForType('longText')).toBe('text');
    expect(categoryForType('checkbox')).toBe('boolean');
    expect(categoryForType('date')).toBe('date');
    expect(categoryForType('createdTime')).toBe('date');
    expect(categoryForType('singleSelect')).toBe('select');
    expect(categoryForType('user')).toBe('select');
    expect(categoryForType('link')).toBe('select');
    expect(categoryForType('multipleSelect')).toBe('multiSelect');
    expect(categoryForType('attachment')).toBe('attachment');
    expect(categoryForType(undefined)).toBe('text');
    expect(categoryForType('something_unknown')).toBe('text');
  });

  it('text fields get Teable text operators, not numeric comparison', () => {
    const ops = operatorsForType('singleLineText').map((o) => o.value);
    expect(ops).toEqual(
      expect.arrayContaining(['is', 'isNot', 'contains', 'doesNotContain', 'isEmpty', 'isNotEmpty'])
    );
    expect(ops).not.toContain('isGreater');
  });

  it('number fields get comparison operators, not contains', () => {
    const ops = operatorsForType('number').map((o) => o.value);
    expect(ops).toEqual(
      expect.arrayContaining(['is', 'isNot', 'isGreater', 'isGreaterEqual', 'isLess', 'isLessEqual'])
    );
    expect(ops).not.toContain('contains');
  });

  it('boolean (checkbox) fields only get the "is" operator', () => {
    const ops = operatorsForType('checkbox').map((o) => o.value);
    expect(ops).toEqual(['is']);
  });

  it('date fields get before/after operators', () => {
    const ops = operatorsForType('date').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['is', 'isBefore', 'isAfter', 'isOnOrBefore', 'isOnOrAfter']));
  });

  it('single-select fields get isAnyOf/isNoneOf list operators', () => {
    const ops = operatorsForType('singleSelect').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['is', 'isNot', 'isAnyOf', 'isNoneOf']));
  });

  it('multi-select fields get hasAnyOf/hasAllOf/hasNoneOf operators', () => {
    const ops = operatorsForType('multipleSelect').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['hasAnyOf', 'hasAllOf', 'hasNoneOf', 'isExactly']));
  });

  it('attachment fields only get emptiness checks', () => {
    const ops = operatorsForType('attachment').map((o) => o.value);
    expect(ops).toEqual(['isEmpty', 'isNotEmpty']);
  });

  it('reports operator arity', () => {
    expect(operatorArity('is')).toBe('single');
    expect(operatorArity('contains')).toBe('single');
    expect(operatorArity('isBefore')).toBe('single');
    expect(operatorArity('isEmpty')).toBe('none');
    expect(operatorArity('isNotEmpty')).toBe('none');
    expect(operatorArity('isAnyOf')).toBe('list');
    expect(operatorArity('hasAllOf')).toBe('list');
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
      children: [{ kind: 'condition', field: 'Plan', category: 'text', op: 'is', value: 'pro' }],
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
        { kind: 'condition', field: 'Plan', category: 'text', op: 'is', value: '$plan' },
        {
          kind: 'group',
          connector: 'or',
          children: [{ kind: 'condition', field: 'Owner', category: 'select', op: 'isAnyOf', value: '$user' }],
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

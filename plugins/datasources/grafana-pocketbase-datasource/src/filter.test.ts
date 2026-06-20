import {
  categoryForType,
  emptyRootGroup,
  interpolateFilterTree,
  operatorArity,
  operatorsForType,
  parseFilterTree,
  stringifyFilterTree,
  FilterGroup,
} from './filter';

describe('operator catalog', () => {
  it('maps PocketBase field types to categories', () => {
    expect(categoryForType('text')).toBe('text');
    expect(categoryForType('email')).toBe('text');
    expect(categoryForType('select')).toBe('text');
    expect(categoryForType('relation')).toBe('text');
    expect(categoryForType('number')).toBe('number');
    expect(categoryForType('bool')).toBe('boolean');
    expect(categoryForType('date')).toBe('datetime');
    expect(categoryForType('autodate')).toBe('datetime');
    expect(categoryForType(undefined)).toBe('text');
    expect(categoryForType('something-new')).toBe('text');
  });

  it('returns type-appropriate operators', () => {
    const text = operatorsForType('text').map((o) => o.value);
    expect(text).toContain('contains');
    expect(text).toContain('notContains');
    expect(text).not.toContain('greaterThan');

    const num = operatorsForType('number').map((o) => o.value);
    expect(num).toContain('greaterThan');
    expect(num).not.toContain('contains');

    const bool = operatorsForType('bool').map((o) => o.value);
    expect(bool).toContain('equal');
    expect(bool).not.toContain('greaterThan');
  });

  it('reports operator arity', () => {
    expect(operatorArity('equal')).toBe('single');
    expect(operatorArity('isNull')).toBe('none');
    expect(operatorArity('isNotNull')).toBe('none');
    expect(operatorArity('unknown')).toBe('single');
  });
});

describe('filter tree serialization', () => {
  it('parses an empty/undefined tree to an empty root group', () => {
    expect(parseFilterTree(undefined)).toEqual(emptyRootGroup());
    expect(parseFilterTree('')).toEqual(emptyRootGroup());
    expect(parseFilterTree('not-json')).toEqual(emptyRootGroup());
  });

  it('round-trips a populated tree', () => {
    const tree: FilterGroup = {
      kind: 'group',
      connector: 'and',
      children: [{ kind: 'condition', attribute: 'status', op: 'equal', value: 'active' }],
    };
    const serialized = stringifyFilterTree(tree);
    expect(parseFilterTree(serialized)).toEqual(tree);
  });

  it('serializes an empty tree to an empty string', () => {
    expect(stringifyFilterTree(emptyRootGroup())).toBe('');
  });

  it('interpolates condition values recursively', () => {
    const tree: FilterGroup = {
      kind: 'group',
      connector: 'and',
      children: [
        { kind: 'condition', attribute: 'status', op: 'equal', value: '$status' },
        {
          kind: 'group',
          connector: 'or',
          children: [{ kind: 'condition', attribute: 'tier', op: 'equal', value: '$tier' }],
        },
      ],
    };
    const result = interpolateFilterTree(tree, (v) => v.replace('$status', 'active').replace('$tier', 'pro'));
    expect((result.children[0] as any).value).toBe('active');
    expect(((result.children[1] as FilterGroup).children[0] as any).value).toBe('pro');
  });
});

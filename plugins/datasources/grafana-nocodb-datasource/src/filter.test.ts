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
  it('maps uidt to categories', () => {
    expect(categoryForType('Number')).toBe('number');
    expect(categoryForType('SingleLineText')).toBe('text');
    expect(categoryForType('Checkbox')).toBe('boolean');
    expect(categoryForType('DateTime')).toBe('date');
    expect(categoryForType('MultiSelect')).toBe('array');
    expect(categoryForType('SingleSelect')).toBe('select');
    expect(categoryForType(undefined)).toBe('text');
    expect(categoryForType('SomethingUnknown')).toBe('text');
  });

  it('text fields get like/in but not gt/btw', () => {
    const ops = operatorsForType('SingleLineText').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['eq', 'neq', 'like', 'nlike', 'in', 'blank', 'notblank']));
    expect(ops).not.toContain('gt');
    expect(ops).not.toContain('btw');
  });

  it('number fields get comparison, not like (NocoDB rejects btw for numbers)', () => {
    const ops = operatorsForType('Number').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['eq', 'gt', 'ge', 'lt', 'le']));
    expect(ops).not.toContain('like');
    expect(ops).not.toContain('btw');
  });

  it('boolean fields only get equality', () => {
    expect(operatorsForType('Checkbox').map((o) => o.value)).toEqual(['eq']);
  });

  it('array fields get any/all-of operators', () => {
    const ops = operatorsForType('MultiSelect').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['anyof', 'allof', 'nanyof', 'nallof']));
  });

  it('date fields include isWithin', () => {
    expect(operatorsForType('DateTime').map((o) => o.value)).toContain('isWithin');
  });

  it('reports operator arity', () => {
    expect(operatorArity('blank')).toBe('none');
    expect(operatorArity('eq')).toBe('single');
    expect(operatorArity('in')).toBe('list');
  });
});

describe('interpolateFilterTree', () => {
  it('interpolates single values without list formatting', () => {
    const root: FilterGroup = {
      kind: 'group',
      connector: 'and',
      children: [{ kind: 'condition', field: 'Name', op: 'eq', value: '$name' }],
    };
    const calls: Array<{ value: string; asList: boolean }> = [];
    const out = interpolateFilterTree(root, (value, asList) => {
      calls.push({ value, asList });
      return value === '$name' ? 'Alice' : value;
    });
    expect(out.children[0]).toMatchObject({ field: 'Name', value: 'Alice' });
    expect(calls).toEqual([{ value: '$name', asList: false }]);
  });

  it('flags list operators so the caller can use csv formatting', () => {
    const root: FilterGroup = {
      kind: 'group',
      connector: 'and',
      children: [{ kind: 'condition', field: 'Status', op: 'in', value: '$statuses' }],
    };
    const seen: boolean[] = [];
    interpolateFilterTree(root, (value, asList) => {
      seen.push(asList);
      return value;
    });
    expect(seen).toEqual([true]);
  });

  it('interpolates values inside nested groups and skips empty values', () => {
    const root: FilterGroup = {
      kind: 'group',
      connector: 'and',
      children: [
        { kind: 'condition', field: 'A', op: 'eq', value: '' },
        {
          kind: 'group',
          connector: 'or',
          children: [{ kind: 'condition', field: 'B', op: 'eq', value: '$b' }],
        },
      ],
    };
    const out = interpolateFilterTree(root, (v) => (v === '$b' ? 'x' : v));
    // empty value untouched, nested value interpolated
    expect((out.children[1] as FilterGroup).children[0]).toMatchObject({ value: 'x' });
  });
});

describe('filter tree persistence', () => {
  it('round-trips', () => {
    const root: FilterGroup = {
      kind: 'group',
      connector: 'or',
      children: [newCondition()],
    };
    const json = stringifyFilterTree(root);
    expect(parseFilterTree(json)).toEqual(root);
  });

  it('empty tree stringifies to empty string', () => {
    expect(stringifyFilterTree(emptyRootGroup())).toBe('');
    expect(parseFilterTree('')).toEqual(emptyRootGroup());
  });

  it('tolerates malformed json', () => {
    expect(parseFilterTree('not json')).toEqual(emptyRootGroup());
  });
});

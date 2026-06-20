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
  it('maps Baserow field types to categories', () => {
    expect(categoryForType('number')).toBe('number');
    expect(categoryForType('text')).toBe('text');
    expect(categoryForType('boolean')).toBe('boolean');
    expect(categoryForType('date')).toBe('date');
    expect(categoryForType('multiple_select')).toBe('multi_select');
    expect(categoryForType('single_select')).toBe('single_select');
    expect(categoryForType('link_row')).toBe('link_row');
    expect(categoryForType(undefined)).toBe('text');
    expect(categoryForType('something_unknown')).toBe('text');
  });

  it('text fields get contains operators but not numeric comparison', () => {
    const ops = operatorsForType('text').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['equal', 'not_equal', 'contains', 'empty', 'not_empty']));
    expect(ops).not.toContain('higher_than');
  });

  it('number fields get comparison, not contains', () => {
    const ops = operatorsForType('number').map((o) => o.value);
    expect(ops).toEqual(
      expect.arrayContaining(['equal', 'higher_than', 'higher_than_or_equal', 'lower_than', 'lower_than_or_equal'])
    );
    expect(ops).not.toContain('contains');
  });

  it('boolean fields only get the boolean operator', () => {
    expect(operatorsForType('boolean').map((o) => o.value)).toEqual(['boolean']);
  });

  it('single select fields get is-any-of operators', () => {
    const ops = operatorsForType('single_select').map((o) => o.value);
    expect(ops).toEqual(
      expect.arrayContaining(['single_select_equal', 'single_select_is_any_of', 'single_select_is_none_of'])
    );
  });

  it('multi select fields get has operators', () => {
    const ops = operatorsForType('multiple_select').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['multiple_select_has', 'multiple_select_has_not']));
  });

  it('date fields include date_is operators', () => {
    expect(operatorsForType('date').map((o) => o.value)).toContain('date_is');
  });

  it('reports operator arity', () => {
    expect(operatorArity('empty')).toBe('none');
    expect(operatorArity('equal')).toBe('single');
    expect(operatorArity('single_select_is_any_of')).toBe('list');
  });
});

describe('interpolateFilterTree', () => {
  it('interpolates single values without list formatting', () => {
    const root: FilterGroup = {
      kind: 'group',
      connector: 'and',
      children: [{ kind: 'condition', field: 'Name', op: 'equal', value: '$name' }],
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
      children: [{ kind: 'condition', field: 'Status', op: 'single_select_is_any_of', value: '$statuses' }],
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
        { kind: 'condition', field: 'A', op: 'equal', value: '' },
        {
          kind: 'group',
          connector: 'or',
          children: [{ kind: 'condition', field: 'B', op: 'equal', value: '$b' }],
        },
      ],
    };
    const out = interpolateFilterTree(root, (v) => (v === '$b' ? 'x' : v));
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

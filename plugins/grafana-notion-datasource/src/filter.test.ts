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
  it('maps Notion types to categories', () => {
    expect(categoryForType('number')).toBe('number');
    expect(categoryForType('title')).toBe('text');
    expect(categoryForType('rich_text')).toBe('text');
    expect(categoryForType('checkbox')).toBe('checkbox');
    expect(categoryForType('date')).toBe('date');
    expect(categoryForType('multi_select')).toBe('multi_select');
    expect(categoryForType('select')).toBe('select');
    expect(categoryForType('status')).toBe('status');
    expect(categoryForType('people')).toBe('people');
    expect(categoryForType(undefined)).toBe('text');
    expect(categoryForType('SomethingUnknown')).toBe('text');
  });

  it('text fields get contains/starts_with but not greater_than', () => {
    const ops = operatorsForType('rich_text').map((o) => o.value);
    expect(ops).toEqual(
      expect.arrayContaining(['equals', 'does_not_equal', 'contains', 'starts_with', 'is_empty', 'is_not_empty'])
    );
    expect(ops).not.toContain('greater_than');
  });

  it('number fields get comparison, not contains', () => {
    const ops = operatorsForType('number').map((o) => o.value);
    expect(ops).toEqual(
      expect.arrayContaining(['equals', 'greater_than', 'greater_than_or_equal_to', 'less_than', 'less_than_or_equal_to'])
    );
    expect(ops).not.toContain('contains');
  });

  it('checkbox fields only get equality', () => {
    expect(operatorsForType('checkbox').map((o) => o.value)).toEqual(['equals']);
  });

  it('select fields get in/not_in list operators', () => {
    const ops = operatorsForType('select').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['in', 'not_in']));
  });

  it('date fields include before/after', () => {
    const ops = operatorsForType('date').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['before', 'after', 'on_or_before', 'on_or_after']));
  });

  it('reports operator arity', () => {
    expect(operatorArity('is_empty')).toBe('none');
    expect(operatorArity('equals')).toBe('single');
    expect(operatorArity('in')).toBe('list');
  });
});

describe('interpolateFilterTree', () => {
  it('interpolates single values without list formatting', () => {
    const root: FilterGroup = {
      kind: 'group',
      connector: 'and',
      children: [{ kind: 'condition', field: 'Name', category: 'text', op: 'equals', value: '$name' }],
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
      children: [{ kind: 'condition', field: 'Status', category: 'select', op: 'in', value: '$statuses' }],
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
        { kind: 'condition', field: 'A', category: 'text', op: 'equals', value: '' },
        {
          kind: 'group',
          connector: 'or',
          children: [{ kind: 'condition', field: 'B', category: 'text', op: 'equals', value: '$b' }],
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

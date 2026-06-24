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
  it('maps SeaTable column types to categories', () => {
    expect(categoryForType('number')).toBe('number');
    expect(categoryForType('rate')).toBe('number');
    expect(categoryForType('text')).toBe('text');
    expect(categoryForType('long-text')).toBe('text');
    expect(categoryForType('single-select')).toBe('text');
    expect(categoryForType('checkbox')).toBe('boolean');
    expect(categoryForType('date')).toBe('date');
    expect(categoryForType('ctime')).toBe('date');
    expect(categoryForType('mtime')).toBe('date');
    expect(categoryForType(undefined)).toBe('text');
    expect(categoryForType('unknown_type')).toBe('text');
  });

  it('text columns get contains/not_contains and list operators', () => {
    const ops = operatorsForType('text').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['eq', 'neq', 'contains', 'not_contains', 'in', 'empty', 'not_empty']));
  });

  it('number columns get comparison, not contains', () => {
    const ops = operatorsForType('number').map((o) => o.value);
    expect(ops).toEqual(expect.arrayContaining(['eq', 'neq', 'gt', 'gte', 'lt', 'lte']));
    expect(ops).not.toContain('contains');
  });

  it('checkbox columns get is_true/is_false', () => {
    const ops = operatorsForType('checkbox').map((o) => o.value);
    expect(ops).toEqual(['is_true', 'is_false']);
  });

  it('reports operator arity', () => {
    expect(operatorArity('eq')).toBe('single');
    expect(operatorArity('empty')).toBe('none');
    expect(operatorArity('is_true')).toBe('none');
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
      children: [{ kind: 'condition', field: 'Status', op: 'eq', value: 'published' }],
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
        { kind: 'condition', field: 'Status', op: 'eq', value: '$status' },
        {
          kind: 'group',
          connector: 'or',
          children: [{ kind: 'condition', field: 'Author', op: 'eq', value: '$author' }],
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

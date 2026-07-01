import {
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
  it('exposes the core PostgREST operators', () => {
    const ops = operatorsForType().map((o) => o.value);
    expect(ops).toEqual(
      expect.arrayContaining([
        'eq',
        'neq',
        'gt',
        'gte',
        'lt',
        'lte',
        'like',
        'ilike',
        'match',
        'imatch',
        'in',
        'cs',
        'cd',
        'is.null',
        'not.is.null',
        'is.true',
        'is.false',
      ])
    );
  });

  it('reports operator arity', () => {
    expect(operatorArity('eq')).toBe('single');
    expect(operatorArity('like')).toBe('single');
    expect(operatorArity('in')).toBe('single');
    expect(operatorArity('is.null')).toBe('none');
    expect(operatorArity('not.is.null')).toBe('none');
    expect(operatorArity('is.true')).toBe('none');
    expect(operatorArity('unknown')).toBe('single');
  });

  it('defaults a new condition to eq', () => {
    expect(newCondition().op).toBe('eq');
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
      children: [{ kind: 'condition', field: 'status', op: 'eq', value: 'active' }],
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
          children: [{ kind: 'condition', field: 'owner', op: 'eq', value: '$user' }],
        },
      ],
    };
    const out = interpolateFilterTree(tree, (v) => v.replace('$status', 'active').replace('$user', 'alice'));
    expect((out.children[0] as any).value).toBe('active');
    expect(((out.children[1] as FilterGroup).children[0] as any).value).toBe('alice');
  });

  it('leaves valueless conditions untouched', () => {
    const c = newCondition();
    const tree: FilterGroup = { kind: 'group', connector: 'and', children: [{ ...c, value: '' }] };
    const out = interpolateFilterTree(tree, () => 'SHOULD_NOT_APPEAR');
    expect((out.children[0] as any).value).toBe('');
  });
});

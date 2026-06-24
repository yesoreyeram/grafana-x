import { INTERCOM_OPERATORS, interpolateFilters, isListOperator, newFilter, operatorOptions, SearchFilter } from './filter';

describe('operator catalog', () => {
  it('exposes the Intercom search operators', () => {
    const values = INTERCOM_OPERATORS.map((o) => o.value);
    expect(values).toEqual(expect.arrayContaining(['=', '!=', '>', '<', '~', '!~', '^', '$', 'IN', 'NIN']));
  });

  it('maps operators to selectable options', () => {
    const opts = operatorOptions();
    expect(opts.find((o) => o.value === '~')).toBeTruthy();
    expect(opts.length).toBe(INTERCOM_OPERATORS.length);
  });

  it('flags list operators', () => {
    expect(isListOperator('IN')).toBe(true);
    expect(isListOperator('NIN')).toBe(true);
    expect(isListOperator('=')).toBe(false);
  });

  it('newFilter has sensible defaults', () => {
    expect(newFilter()).toEqual({ field: '', operator: '=', value: '' });
  });
});

describe('interpolateFilters', () => {
  it('interpolates single values without list formatting', () => {
    const filters: SearchFilter[] = [{ field: 'role', operator: '=', value: '$role' }];
    const seen: boolean[] = [];
    const out = interpolateFilters(filters, (value, asList) => {
      seen.push(asList);
      return value === '$role' ? 'user' : value;
    });
    expect(out?.[0]).toEqual({ field: 'role', operator: '=', value: 'user' });
    expect(seen).toEqual([false]);
  });

  it('flags list operators so the caller can use csv formatting', () => {
    const filters: SearchFilter[] = [{ field: 'tag_ids', operator: 'IN', value: '$tags' }];
    const seen: boolean[] = [];
    interpolateFilters(filters, (value, asList) => {
      seen.push(asList);
      return value;
    });
    expect(seen).toEqual([true]);
  });

  it('skips empty values and tolerates undefined', () => {
    const filters: SearchFilter[] = [{ field: 'state', operator: '=', value: '' }];
    const out = interpolateFilters(filters, () => 'SHOULD_NOT_BE_CALLED');
    expect(out?.[0].value).toBe('');
    expect(interpolateFilters(undefined, () => 'x')).toBeUndefined();
  });
});

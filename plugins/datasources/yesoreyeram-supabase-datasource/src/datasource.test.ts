import { DataSource } from './datasource';
import { SupabaseQuery } from './types';

jest.mock('@grafana/runtime', () => {
  const actual = jest.requireActual('@grafana/runtime');
  return {
    ...actual,
    getTemplateSrv: () => ({
      replace: (s: string) => (s ? s.replace(/\$table/g, 'users').replace(/\$status/g, 'active') : s),
    }),
  };
});

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-supabase-datasource',
    name: 'Supabase',
    jsonData: { apiUrl: 'https://xxx.supabase.co/rest/v1' },
  } as any);

describe('Supabase DataSource', () => {
  it('getTables fetches via the tables resource', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ tables: [{ id: 'users', title: 'users' }] });
    (ds as any).getResource = getResource;

    const tables = await ds.getTables();
    expect(getResource).toHaveBeenCalledWith('tables');
    expect(tables).toEqual([{ id: 'users', title: 'users' }]);
  });

  it('getTables returns an empty list when the resource has none', async () => {
    const ds = makeDS();
    (ds as any).getResource = jest.fn().mockResolvedValue({});
    await expect(ds.getTables()).resolves.toEqual([]);
  });

  it('filterQuery requires a tableId', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'records' } as SupabaseQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'records', tableId: 'users' } as SupabaseQuery)).toBe(true);
  });

  it('applyTemplateVariables interpolates table, select and filter values', () => {
    const ds = makeDS();
    const query: SupabaseQuery = {
      refId: 'A',
      queryType: 'records',
      tableId: '$table',
      select: '$status',
      filterTree: JSON.stringify({
        kind: 'group',
        connector: 'and',
        children: [{ kind: 'condition', field: 'status', op: 'eq', value: '$status' }],
      }),
    };

    const out = ds.applyTemplateVariables(query, {});
    expect(out.tableId).toBe('users');
    expect(out.select).toBe('active');
    const tree = JSON.parse(out.filterTree as string);
    expect(tree.children[0].value).toBe('active');
  });
});

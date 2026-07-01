import { DataSource } from './datasource';
import { SeaTableQuery } from './types';

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-seatable-datasource',
    name: 'SeaTable',
    jsonData: { serverURL: 'https://cloud.seatable.io' },
  } as any);

describe('SeaTable DataSource', () => {
  it('getTables fetches via the tables resource', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      tables: [{ name: 'Table1', columns: [{ name: 'Name', key: '0000', type: 'text' }] }],
    });
    (ds as any).getResource = getResource;

    const tables = await ds.getTables();
    expect(getResource).toHaveBeenCalledWith('tables');
    expect(tables).toEqual([{ name: 'Table1', columns: [{ name: 'Name', key: '0000', type: 'text' }] }]);
  });

  it('filterQuery requires a table name for records/count', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'records' } as SeaTableQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'records', tableName: 'Table1' } as SeaTableQuery)).toBe(true);
    expect(ds.filterQuery({ refId: 'A', queryType: 'count', tableName: 'Table1' } as SeaTableQuery)).toBe(true);
  });

  it('filterQuery requires sql for the sql query type', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'sql' } as SeaTableQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'sql', sql: '  ' } as SeaTableQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'sql', sql: 'SELECT 1' } as SeaTableQuery)).toBe(true);
  });
});

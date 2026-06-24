import { DataSource } from './datasource';
import { TeableQuery } from './types';

const makeDS = (defaultBaseId?: string) =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-teable-datasource',
    name: 'Teable',
    jsonData: { baseURL: 'https://app.teable.io', defaultBaseId },
  } as any);

describe('Teable DataSource', () => {
  it('reads defaultBaseId from instance settings', () => {
    expect(makeDS('bse123').defaultBaseId).toBe('bse123');
    expect(makeDS().defaultBaseId).toBe('');
  });

  it('filterQuery only requires a tableId', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'records' } as TeableQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'records', tableId: 'tbl1' } as TeableQuery)).toBe(true);
  });

  it('getTables uses the configured default base when no baseId is given', async () => {
    const ds = makeDS('bse123');
    const getResource = jest.fn().mockResolvedValue({ tables: [{ id: 'tbl1', name: 'Users' }] });
    (ds as any).getResource = getResource;

    const tables = await ds.getTables();
    expect(getResource).toHaveBeenCalledWith('tables', { baseId: 'bse123' });
    expect(tables).toEqual([{ id: 'tbl1', name: 'Users' }]);
  });

  it('getTables scopes to a base when baseId is given', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ tables: [] });
    (ds as any).getResource = getResource;

    await ds.getTables('bseX');
    expect(getResource).toHaveBeenCalledWith('tables', { baseId: 'bseX' });
  });

  it('getTables returns empty without any base id', async () => {
    const ds = makeDS();
    const getResource = jest.fn();
    (ds as any).getResource = getResource;

    await expect(ds.getTables()).resolves.toEqual([]);
    expect(getResource).not.toHaveBeenCalled();
  });

  it('getFields fetches a table fields', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      fields: [
        { id: 'fld1', name: 'Name', type: 'singleLineText' },
        { id: 'fld2', name: 'Age', type: 'number' },
      ],
    });
    (ds as any).getResource = getResource;

    const fields = await ds.getFields('tbl1');
    expect(getResource).toHaveBeenCalledWith('fields', { tableId: 'tbl1' });
    expect(fields).toEqual([
      { id: 'fld1', name: 'Name', type: 'singleLineText' },
      { id: 'fld2', name: 'Age', type: 'number' },
    ]);
  });

  it('getFields returns empty without tableId', async () => {
    const ds = makeDS();
    const getResource = jest.fn();
    (ds as any).getResource = getResource;

    await expect(ds.getFields('')).resolves.toEqual([]);
    expect(getResource).not.toHaveBeenCalled();
  });
});

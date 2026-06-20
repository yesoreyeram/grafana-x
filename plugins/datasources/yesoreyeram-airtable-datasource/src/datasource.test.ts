import { DataSource } from './datasource';
import { AirtableQuery } from './types';

const makeDS = (baseId?: string) =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-airtable-datasource',
    name: 'Airtable',
    jsonData: { baseURL: 'https://api.airtable.com', baseId },
  } as any);

describe('Airtable DataSource', () => {
  it('reads baseId from instance settings', () => {
    expect(makeDS('appABC').baseId).toBe('appABC');
    expect(makeDS().baseId).toBe('');
  });

  it('getBases fetches via the bases resource', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ bases: [{ id: 'app1', title: 'Sales' }] });
    (ds as any).getResource = getResource;

    const bases = await ds.getBases();
    expect(getResource).toHaveBeenCalledWith('bases');
    expect(bases).toEqual([{ id: 'app1', title: 'Sales' }]);
  });

  it('filterQuery requires a tableId and a base', () => {
    const withBase = makeDS('appABC');
    expect(withBase.filterQuery({ refId: 'A', queryType: 'records' } as AirtableQuery)).toBe(false);
    expect(withBase.filterQuery({ refId: 'A', queryType: 'records', tableId: 'tbl1' } as AirtableQuery)).toBe(true);

    const noBase = makeDS();
    expect(noBase.filterQuery({ refId: 'A', queryType: 'records', tableId: 'tbl1' } as AirtableQuery)).toBe(false);
    expect(
      noBase.filterQuery({ refId: 'A', queryType: 'records', tableId: 'tbl1', baseId: 'appX' } as AirtableQuery)
    ).toBe(true);
  });

  it('getTables uses the configured base when no baseId is given', async () => {
    const ds = makeDS('appABC');
    const getResource = jest.fn().mockResolvedValue({ tables: [{ id: 'tbl1', title: 'Users' }] });
    (ds as any).getResource = getResource;

    const tables = await ds.getTables();
    expect(getResource).toHaveBeenCalledWith('tables', undefined);
    expect(tables).toEqual([{ id: 'tbl1', title: 'Users' }]);
  });

  it('getTables scopes to a base when baseId is given', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ tables: [] });
    (ds as any).getResource = getResource;

    await ds.getTables('appX');
    expect(getResource).toHaveBeenCalledWith('tables', { baseId: 'appX' });
  });

  it('getFields fetches a table fields and forwards baseId when given', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      fields: [
        { title: 'Name', type: 'singleLineText' },
        { title: 'Age', type: 'number' },
      ],
    });
    (ds as any).getResource = getResource;

    const fields = await ds.getFields('tbl1', 'appX');
    expect(getResource).toHaveBeenCalledWith('fields', { tableId: 'tbl1', baseId: 'appX' });
    expect(fields).toEqual([
      { title: 'Name', type: 'singleLineText' },
      { title: 'Age', type: 'number' },
    ]);
  });

  it('getFields returns empty without tableId', async () => {
    const ds = makeDS();
    const getResource = jest.fn();
    (ds as any).getResource = getResource;

    await expect(ds.getFields('')).resolves.toEqual([]);
    expect(getResource).not.toHaveBeenCalled();
  });

  it('getViews fetches a table views', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ views: [{ id: 'viw1', title: 'Grid' }] });
    (ds as any).getResource = getResource;

    const views = await ds.getViews('tbl1');
    expect(getResource).toHaveBeenCalledWith('views', { tableId: 'tbl1' });
    expect(views).toEqual([{ id: 'viw1', title: 'Grid' }]);
  });

  it('getViews returns empty without tableId', async () => {
    const ds = makeDS();
    const getResource = jest.fn();
    (ds as any).getResource = getResource;

    await expect(ds.getViews('')).resolves.toEqual([]);
    expect(getResource).not.toHaveBeenCalled();
  });
});

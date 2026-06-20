import { DataSource } from './datasource';
import { NocoDBQuery } from './types';

const makeDS = (baseId?: string) =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-nocodb-datasource',
    name: 'NocoDB',
    jsonData: { baseURL: 'https://app.nocodb.com', baseId },
  } as any);

describe('NocoDB DataSource', () => {
  it('reads baseId from instance settings', () => {
    expect(makeDS('p_abc').baseId).toBe('p_abc');
    expect(makeDS().baseId).toBe('');
  });

  it('filterQuery requires a tableId', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'records' } as NocoDBQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'records', tableId: 'm_1' } as NocoDBQuery)).toBe(true);
  });

  it('getTables fetches all tables when no baseId is given', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      tables: [{ id: 'm_1', title: 'Users', baseId: 'p_1', baseTitle: 'Sales' }],
    });
    (ds as any).getResource = getResource;

    const tables = await ds.getTables();
    expect(getResource).toHaveBeenCalledWith('tables', undefined);
    expect(tables).toEqual([{ id: 'm_1', title: 'Users', baseId: 'p_1', baseTitle: 'Sales' }]);
  });

  it('getTables scopes to a base when baseId is given', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ tables: [] });
    (ds as any).getResource = getResource;

    await ds.getTables('p_2');
    expect(getResource).toHaveBeenCalledWith('tables', { baseId: 'p_2' });
  });

  it('getFields fetches a table fields', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      fields: [{ title: 'Title', type: 'SingleLineText' }, { title: 'Age', type: 'Number' }],
    });
    (ds as any).getResource = getResource;

    const fields = await ds.getFields('m_1');
    expect(getResource).toHaveBeenCalledWith('fields', { tableId: 'm_1' });
    expect(fields).toEqual([
      { title: 'Title', type: 'SingleLineText' },
      { title: 'Age', type: 'Number' },
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
    const getResource = jest.fn().mockResolvedValue({
      views: [{ id: 'vw_1', title: 'Grid' }],
    });
    (ds as any).getResource = getResource;

    const views = await ds.getViews('m_1');
    expect(getResource).toHaveBeenCalledWith('views', { tableId: 'm_1' });
    expect(views).toEqual([{ id: 'vw_1', title: 'Grid' }]);
  });

  it('getViews returns empty without tableId', async () => {
    const ds = makeDS();
    const getResource = jest.fn();
    (ds as any).getResource = getResource;

    await expect(ds.getViews('')).resolves.toEqual([]);
    expect(getResource).not.toHaveBeenCalled();
  });
});

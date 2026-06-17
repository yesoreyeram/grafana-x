import { DataSource } from './datasource';
import { BaserowQuery } from './types';

const makeDS = (databaseId?: string, jsonExtra: Record<string, unknown> = {}) =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-baserow-datasource',
    name: 'Baserow',
    jsonData: { baseURL: 'https://api.baserow.io', databaseId, ...jsonExtra },
  } as any);

describe('Baserow DataSource', () => {
  it('reads databaseId from instance settings', () => {
    expect(makeDS('42').databaseId).toBe('42');
    expect(makeDS().databaseId).toBe('');
  });

  it('defaults authMode to token and reads password mode from settings', () => {
    expect(makeDS().authMode).toBe('token');
    expect(makeDS(undefined, { authMode: 'password' }).authMode).toBe('password');
  });

  it('getDatabases fetches via the databases resource', async () => {
    const ds = makeDS(undefined, { authMode: 'password' });
    const getResource = jest.fn().mockResolvedValue({
      databases: [{ id: '11', title: 'Sales', workspaceName: 'Acme' }],
    });
    (ds as any).getResource = getResource;

    const dbs = await ds.getDatabases();
    expect(getResource).toHaveBeenCalledWith('databases');
    expect(dbs).toEqual([{ id: '11', title: 'Sales', workspaceName: 'Acme' }]);
  });

  it('filterQuery requires a tableId', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'records' } as BaserowQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'records', tableId: '1' } as BaserowQuery)).toBe(true);
  });

  it('getTables uses the configured database when no databaseId is given', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      tables: [{ id: '11', title: 'Users', databaseId: '42' }],
    });
    (ds as any).getResource = getResource;

    const tables = await ds.getTables();
    expect(getResource).toHaveBeenCalledWith('tables', undefined);
    expect(tables).toEqual([{ id: '11', title: 'Users', databaseId: '42' }]);
  });

  it('getTables scopes to a database when databaseId is given', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ tables: [] });
    (ds as any).getResource = getResource;

    await ds.getTables('9');
    expect(getResource).toHaveBeenCalledWith('tables', { databaseId: '9' });
  });

  it('getFields fetches a table fields', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      fields: [
        { title: 'Name', type: 'text' },
        { title: 'Age', type: 'number' },
      ],
    });
    (ds as any).getResource = getResource;

    const fields = await ds.getFields('11');
    expect(getResource).toHaveBeenCalledWith('fields', { tableId: '11' });
    expect(fields).toEqual([
      { title: 'Name', type: 'text' },
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
    const getResource = jest.fn().mockResolvedValue({
      views: [{ id: '21', title: 'Grid' }],
    });
    (ds as any).getResource = getResource;

    const views = await ds.getViews('11');
    expect(getResource).toHaveBeenCalledWith('views', { tableId: '11' });
    expect(views).toEqual([{ id: '21', title: 'Grid' }]);
  });

  it('getViews returns empty without tableId', async () => {
    const ds = makeDS();
    const getResource = jest.fn();
    (ds as any).getResource = getResource;

    await expect(ds.getViews('')).resolves.toEqual([]);
    expect(getResource).not.toHaveBeenCalled();
  });
});

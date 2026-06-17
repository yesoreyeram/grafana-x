import { DataSource } from './datasource';
import { NotionQuery } from './types';

const makeDS = (databaseId?: string) =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-notion-datasource',
    name: 'Notion',
    jsonData: { baseURL: 'https://api.notion.com', databaseId },
  } as any);

describe('Notion DataSource', () => {
  it('reads databaseId from instance settings', () => {
    expect(makeDS('db_abc').databaseId).toBe('db_abc');
    expect(makeDS().databaseId).toBe('');
  });

  it('filterQuery requires a databaseId', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'records' } as NotionQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'records', databaseId: 'db_1' } as NotionQuery)).toBe(true);
  });

  it('getDatabases fetches all databases', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      databases: [{ id: 'db_1', title: 'Customers' }],
    });
    (ds as any).getResource = getResource;

    const databases = await ds.getDatabases();
    expect(getResource).toHaveBeenCalledWith('databases');
    expect(databases).toEqual([{ id: 'db_1', title: 'Customers' }]);
  });

  it('getProperties fetches a database properties', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      properties: [
        { title: 'Name', type: 'title', category: 'text' },
        { title: 'MRR', type: 'number', category: 'number' },
      ],
    });
    (ds as any).getResource = getResource;

    const properties = await ds.getProperties('db_1');
    expect(getResource).toHaveBeenCalledWith('properties', { databaseId: 'db_1' });
    expect(properties).toEqual([
      { title: 'Name', type: 'title', category: 'text' },
      { title: 'MRR', type: 'number', category: 'number' },
    ]);
  });

  it('getProperties returns empty without databaseId', async () => {
    const ds = makeDS();
    const getResource = jest.fn();
    (ds as any).getResource = getResource;

    await expect(ds.getProperties('')).resolves.toEqual([]);
    expect(getResource).not.toHaveBeenCalled();
  });
});

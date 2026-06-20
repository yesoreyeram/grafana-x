import { DataSource } from './datasource';
import { AppwriteQuery } from './types';

const makeDS = (databaseId?: string) =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-appwrite-datasource',
    name: 'Appwrite',
    jsonData: { endpoint: 'https://cloud.appwrite.io/v1', projectId: 'proj', databaseId },
  } as any);

describe('Appwrite DataSource', () => {
  it('reads databaseId from instance settings', () => {
    expect(makeDS('dbABC').databaseId).toBe('dbABC');
    expect(makeDS().databaseId).toBe('');
  });

  it('getDatabases fetches via the databases resource', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ databases: [{ id: 'db1', name: 'Sales' }] });
    (ds as any).getResource = getResource;

    const databases = await ds.getDatabases();
    expect(getResource).toHaveBeenCalledWith('databases');
    expect(databases).toEqual([{ id: 'db1', name: 'Sales' }]);
  });

  it('filterQuery requires a collectionId and a database', () => {
    const withDatabase = makeDS('dbABC');
    expect(withDatabase.filterQuery({ refId: 'A', queryType: 'documents' } as AppwriteQuery)).toBe(false);
    expect(
      withDatabase.filterQuery({ refId: 'A', queryType: 'documents', collectionId: 'col1' } as AppwriteQuery)
    ).toBe(true);

    const noDatabase = makeDS();
    expect(noDatabase.filterQuery({ refId: 'A', queryType: 'documents', collectionId: 'col1' } as AppwriteQuery)).toBe(
      false
    );
    expect(
      noDatabase.filterQuery({
        refId: 'A',
        queryType: 'documents',
        collectionId: 'col1',
        databaseId: 'dbX',
      } as AppwriteQuery)
    ).toBe(true);
  });

  it('getCollections uses the configured database when no databaseId is given', async () => {
    const ds = makeDS('dbABC');
    const getResource = jest.fn().mockResolvedValue({ collections: [{ id: 'col1', name: 'Users' }] });
    (ds as any).getResource = getResource;

    const collections = await ds.getCollections();
    expect(getResource).toHaveBeenCalledWith('collections', undefined);
    expect(collections).toEqual([{ id: 'col1', name: 'Users' }]);
  });

  it('getCollections scopes to a database when databaseId is given', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ collections: [] });
    (ds as any).getResource = getResource;

    await ds.getCollections('dbX');
    expect(getResource).toHaveBeenCalledWith('collections', { databaseId: 'dbX' });
  });

  it('getAttributes fetches a collection attributes and forwards databaseId when given', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      attributes: [
        { key: 'name', type: 'string' },
        { key: 'age', type: 'integer' },
      ],
    });
    (ds as any).getResource = getResource;

    const attrs = await ds.getAttributes('col1', 'dbX');
    expect(getResource).toHaveBeenCalledWith('attributes', { collectionId: 'col1', databaseId: 'dbX' });
    expect(attrs).toEqual([
      { key: 'name', type: 'string' },
      { key: 'age', type: 'integer' },
    ]);
  });

  it('getAttributes returns empty without collectionId', async () => {
    const ds = makeDS();
    const getResource = jest.fn();
    (ds as any).getResource = getResource;

    await expect(ds.getAttributes('')).resolves.toEqual([]);
    expect(getResource).not.toHaveBeenCalled();
  });
});

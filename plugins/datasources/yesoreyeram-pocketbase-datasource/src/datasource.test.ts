import { DataSource } from './datasource';
import { PocketBaseQuery } from './types';

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-pocketbase-datasource',
    name: 'PocketBase',
    jsonData: { url: 'http://127.0.0.1:8090', authMode: 'superuser', identity: 'admin@example.com' },
  } as any);

describe('PocketBase DataSource', () => {
  it('getCollections fetches via the collections resource', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      collections: [{ id: 'c1', name: 'demo', type: 'base' }],
    });
    (ds as any).getResource = getResource;

    const collections = await ds.getCollections();
    expect(getResource).toHaveBeenCalledWith('collections');
    expect(collections).toEqual([{ id: 'c1', name: 'demo', type: 'base' }]);
  });

  it('filterQuery requires a collectionId', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'records' } as PocketBaseQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'records', collectionId: 'demo' } as PocketBaseQuery)).toBe(true);
  });

  it('getFields fetches a collection fields and forwards collectionId', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      fields: [
        { name: 'title', type: 'text' },
        { name: 'score', type: 'number' },
      ],
    });
    (ds as any).getResource = getResource;

    const fields = await ds.getFields('demo');
    expect(getResource).toHaveBeenCalledWith('fields', { collectionId: 'demo' });
    expect(fields).toEqual([
      { name: 'title', type: 'text' },
      { name: 'score', type: 'number' },
    ]);
  });

  it('getFields returns empty without collectionId', async () => {
    const ds = makeDS();
    const getResource = jest.fn();
    (ds as any).getResource = getResource;

    await expect(ds.getFields('')).resolves.toEqual([]);
    expect(getResource).not.toHaveBeenCalled();
  });
});

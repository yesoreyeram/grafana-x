import { DataSource } from './datasource';
import { DirectusQuery } from './types';

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-directus-datasource',
    name: 'Directus',
    jsonData: { baseURL: 'https://directus.example.com' },
  } as any);

describe('Directus DataSource', () => {
  it('reads settings from instance settings', () => {
    const ds = makeDS();
    expect(ds.defaultCollectionId).toBe('');
  });

  it('getCollections fetches via the collections resource', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ collections: [{ collection: 'articles', name: 'Articles' }] });
    (ds as any).getResource = getResource;

    const collections = await ds.getCollections();
    expect(getResource).toHaveBeenCalledWith('collections');
    expect(collections).toEqual([{ collection: 'articles', name: 'Articles' }]);
  });

  it('filterQuery requires a collectionId', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'records' } as DirectusQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'records', collectionId: 'articles' } as DirectusQuery)).toBe(true);
  });

  it('getFields fetches fields for a collection', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      fields: [
        { field: 'title', type: 'string' },
        { field: 'views', type: 'integer' },
      ],
    });
    (ds as any).getResource = getResource;

    const fields = await ds.getFields('articles');
    expect(getResource).toHaveBeenCalledWith('fields', { collectionId: 'articles' });
    expect(fields).toEqual([
      { field: 'title', type: 'string' },
      { field: 'views', type: 'integer' },
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

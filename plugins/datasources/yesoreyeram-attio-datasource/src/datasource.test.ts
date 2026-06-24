import { DataSource } from './datasource';
import { AttioQuery } from './types';

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-attio-datasource',
    name: 'Attio',
    jsonData: { defaultObjectId: '' },
  } as any);

describe('Attio DataSource', () => {
  it('reads settings from instance settings', () => {
    const ds = makeDS();
    expect(ds.defaultObjectId).toBe('');
  });

  it('getObjects fetches via the objects resource', async () => {
    const ds = makeDS();
    const getResource = jest
      .fn()
      .mockResolvedValue({ objects: [{ api_slug: 'people', plural_noun: 'People' }] });
    (ds as any).getResource = getResource;

    const objects = await ds.getObjects();
    expect(getResource).toHaveBeenCalledWith('objects');
    expect(objects).toEqual([{ api_slug: 'people', plural_noun: 'People' }]);
  });

  it('filterQuery requires an objectId', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'records' } as AttioQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'records', objectId: 'people' } as AttioQuery)).toBe(true);
  });

  it('getAttributes fetches attributes for an object', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      attributes: [
        { api_slug: 'name', type: 'personal-name' },
        { api_slug: 'score', type: 'number' },
      ],
    });
    (ds as any).getResource = getResource;

    const attrs = await ds.getAttributes('people');
    expect(getResource).toHaveBeenCalledWith('attributes', { objectId: 'people' });
    expect(attrs).toEqual([
      { api_slug: 'name', type: 'personal-name' },
      { api_slug: 'score', type: 'number' },
    ]);
  });

  it('getAttributes returns empty without objectId', async () => {
    const ds = makeDS();
    const getResource = jest.fn();
    (ds as any).getResource = getResource;

    await expect(ds.getAttributes('')).resolves.toEqual([]);
    expect(getResource).not.toHaveBeenCalled();
  });
});

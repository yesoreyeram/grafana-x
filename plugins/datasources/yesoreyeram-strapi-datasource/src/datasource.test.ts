import { DataSource } from './datasource';
import { StrapiQuery } from './types';

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-strapi-datasource',
    name: 'Strapi',
    jsonData: { baseURL: 'https://strapi.example.com', apiVersion: 'v5' },
  } as any);

describe('Strapi DataSource', () => {
  it('reads settings from instance settings', () => {
    const ds = makeDS();
    expect(ds.defaultContentTypeId).toBe('');
  });

  it('getContentTypes fetches via the content-types resource', async () => {
    const ds = makeDS();
    const getResource = jest
      .fn()
      .mockResolvedValue({ contentTypes: [{ uid: 'api::article.article', apiID: 'article', displayName: 'Article', pluralName: 'articles' }] });
    (ds as any).getResource = getResource;

    const contentTypes = await ds.getContentTypes();
    expect(getResource).toHaveBeenCalledWith('content-types');
    expect(contentTypes).toEqual([
      { uid: 'api::article.article', apiID: 'article', displayName: 'Article', pluralName: 'articles' },
    ]);
  });

  it('getContentTypes degrades to an empty list', async () => {
    const ds = makeDS();
    (ds as any).getResource = jest.fn().mockResolvedValue({ contentTypes: [] });
    await expect(ds.getContentTypes()).resolves.toEqual([]);
  });

  it('filterQuery requires a contentTypeId', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'records' } as StrapiQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'records', contentTypeId: 'articles' } as StrapiQuery)).toBe(true);
  });

  it('getFields fetches fields for a content type', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      fields: [
        { field: 'title', type: 'string' },
        { field: 'views', type: 'integer' },
      ],
    });
    (ds as any).getResource = getResource;

    const fields = await ds.getFields('articles');
    expect(getResource).toHaveBeenCalledWith('fields', { contentTypeId: 'articles' });
    expect(fields).toEqual([
      { field: 'title', type: 'string' },
      { field: 'views', type: 'integer' },
    ]);
  });

  it('getFields returns empty without contentTypeId', async () => {
    const ds = makeDS();
    const getResource = jest.fn();
    (ds as any).getResource = getResource;

    await expect(ds.getFields('')).resolves.toEqual([]);
    expect(getResource).not.toHaveBeenCalled();
  });
});

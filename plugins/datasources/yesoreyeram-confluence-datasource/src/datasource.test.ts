import { DataSource } from './datasource';
import { ConfluenceQuery } from './types';

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-confluence-datasource',
    name: 'Confluence',
    jsonData: { baseURL: 'https://acme.atlassian.net/wiki', authMode: 'basic', email: 'a@b.com' },
  } as any);

describe('Confluence DataSource', () => {
  it('filterQuery requires CQL only for search', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'search' } as ConfluenceQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'search', cql: 'type=page' } as ConfluenceQuery)).toBe(true);
    expect(ds.filterQuery({ refId: 'A', queryType: 'pages' } as ConfluenceQuery)).toBe(true);
    expect(ds.filterQuery({ refId: 'A', queryType: 'blogposts' } as ConfluenceQuery)).toBe(true);
    expect(ds.filterQuery({ refId: 'A', queryType: 'count' } as ConfluenceQuery)).toBe(true);
  });

  it('getDefaultQuery returns pages', () => {
    const ds = makeDS();
    expect(ds.getDefaultQuery({} as any)).toEqual({ queryType: 'pages', limit: 0 });
  });

  it('getSpaces fetches spaces from the resource handler', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      spaces: [{ id: '1', key: 'ENG', name: 'Engineering', type: 'global', status: 'current' }],
    });
    (ds as any).getResource = getResource;

    const spaces = await ds.getSpaces();
    expect(getResource).toHaveBeenCalledWith('spaces');
    expect(spaces).toEqual([{ id: '1', key: 'ENG', name: 'Engineering', type: 'global', status: 'current' }]);
  });

  it('getSpaces returns [] when the handler returns nothing', async () => {
    const ds = makeDS();
    (ds as any).getResource = jest.fn().mockResolvedValue({});
    await expect(ds.getSpaces()).resolves.toEqual([]);
  });
});

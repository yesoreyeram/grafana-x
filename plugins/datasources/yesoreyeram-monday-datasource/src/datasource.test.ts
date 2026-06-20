import { DataSource } from './datasource';
import { MondayQuery } from './types';

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-monday-datasource',
    name: 'monday.com',
    jsonData: { baseURL: 'https://api.monday.com/v2', authMethod: 'apiKey' },
  } as any);

describe('monday.com DataSource', () => {
  it('filterQuery runs boards/users/workspaces/tags unconditionally', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'boards' } as MondayQuery)).toBe(true);
    expect(ds.filterQuery({ refId: 'A', queryType: 'users' } as MondayQuery)).toBe(true);
    expect(ds.filterQuery({ refId: 'A', queryType: 'tags' } as MondayQuery)).toBe(true);
  });

  it('filterQuery requires a board for items and groups', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'items' } as MondayQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'items', boardIds: [] } as MondayQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'items', boardIds: ['1'] } as MondayQuery)).toBe(true);
    expect(ds.filterQuery({ refId: 'A', queryType: 'groups', boardIds: ['1'] } as MondayQuery)).toBe(true);
  });

  it('filterQuery requires a document for raw queries', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw' } as MondayQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw', rawQuery: '   ' } as MondayQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw', rawQuery: 'query{me{id}}' } as MondayQuery)).toBe(true);
  });

  it('getBoards / getWorkspaces fetch lists', async () => {
    const ds = makeDS();
    const getResource = jest
      .fn()
      .mockResolvedValueOnce({ boards: [{ id: '1', name: 'Tasks' }] })
      .mockResolvedValueOnce({ workspaces: [{ id: 'w1', name: 'Main' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getBoards()).toEqual([{ id: '1', name: 'Tasks' }]);
    expect(getResource).toHaveBeenCalledWith('boards');
    expect(await ds.getWorkspaces()).toEqual([{ id: 'w1', name: 'Main' }]);
    expect(getResource).toHaveBeenCalledWith('workspaces');
  });

  it('getGroups passes board ids as repeated params', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ groups: [{ id: 'g1', title: 'Doing' }] });
    (ds as any).getResource = getResource;

    const groups = await ds.getGroups(['1', '2']);
    expect(getResource).toHaveBeenCalledWith('groups', { boardId: ['1', '2'] });
    expect(groups).toEqual([{ id: 'g1', title: 'Doing' }]);
  });

  it('getGroups omits params when no boards given', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ groups: [] });
    (ds as any).getResource = getResource;

    await ds.getGroups([]);
    expect(getResource).toHaveBeenCalledWith('groups', undefined);
  });

  it('getColumns passes board ids', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ columns: [{ id: 'status', title: 'Status' }] });
    (ds as any).getResource = getResource;

    const cols = await ds.getColumns(['1']);
    expect(getResource).toHaveBeenCalledWith('columns', { boardId: ['1'] });
    expect(cols).toEqual([{ id: 'status', title: 'Status' }]);
  });

  it('getDefaultQuery returns items with active state', () => {
    const ds = makeDS();
    const q = ds.getDefaultQuery({} as any);
    expect(q.queryType).toBe('items');
    expect(q.state).toBe('active');
    expect(q.includeColumnValues).toBe(true);
  });
});

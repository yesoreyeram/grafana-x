import { DataSource } from './datasource';
import { TrelloQuery } from './types';

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-trello-datasource',
    name: 'Trello',
    jsonData: {},
  } as any);

describe('Trello DataSource', () => {
  it('filterQuery requires boardId for cards', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'cards' } as TrelloQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'cards', boardId: 'b1' } as TrelloQuery)).toBe(true);
  });

  it('filterQuery requires boardId for count', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'count' } as TrelloQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'count', boardId: 'b1' } as TrelloQuery)).toBe(true);
  });

  it('getLists requires a boardId', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ lists: [{ id: 'l1', name: 'Todo' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getLists('')).toEqual([]);
    expect(getResource).not.toHaveBeenCalled();

    await ds.getLists('b1');
    expect(getResource).toHaveBeenCalledWith('lists', { boardId: 'b1' });
  });

  it('getMembers requires a boardId', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ members: [{ id: 'u1', fullName: 'Alice' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getMembers('')).toEqual([]);
    await ds.getMembers('b1');
    expect(getResource).toHaveBeenCalledWith('members', { boardId: 'b1' });
  });

  it('getLabels requires a boardId', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ labels: [{ id: 'l1', name: 'bug' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getLabels('')).toEqual([]);
    await ds.getLabels('b1');
    expect(getResource).toHaveBeenCalledWith('labels', { boardId: 'b1' });
  });

  it('getBoards fetches boards list', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ boards: [{ id: 'b1', name: 'My Board' }] });
    (ds as any).getResource = getResource;

    const boards = await ds.getBoards();
    expect(getResource).toHaveBeenCalledWith('boards');
    expect(boards).toEqual([{ id: 'b1', name: 'My Board' }]);
  });

  it('getDefaultQuery returns cards with default filter and no limit', () => {
    const ds = makeDS();
    const q = ds.getDefaultQuery({} as any);
    expect(q.queryType).toBe('cards');
    expect(q.cardFilter).toBe('all');
    expect(q.createdMode).toBe('any');
    expect(q.limit).toBe(0);
  });
});

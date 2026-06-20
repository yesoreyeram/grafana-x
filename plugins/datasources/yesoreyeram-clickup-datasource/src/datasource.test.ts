import { DataSource } from './datasource';
import { ClickUpQuery } from './types';

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-clickup-datasource',
    name: 'ClickUp',
    jsonData: { baseURL: 'https://api.clickup.com/api', authMethod: 'apiKey' },
  } as any);

describe('ClickUp DataSource', () => {
  it('filterQuery always runs predefined queries', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'tasks' } as ClickUpQuery)).toBe(true);
    expect(ds.filterQuery({ refId: 'A', queryType: 'spaces' } as ClickUpQuery)).toBe(true);
  });

  it('filterQuery requires a path for raw queries', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw' } as ClickUpQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw', rawPath: '   ' } as ClickUpQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw', rawPath: '/v2/user' } as ClickUpQuery)).toBe(true);
  });

  it('getTeams fetches all workspaces', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ teams: [{ id: '1', name: 'My WS' }] });
    (ds as any).getResource = getResource;

    const teams = await ds.getTeams();
    expect(getResource).toHaveBeenCalledWith('teams');
    expect(teams).toEqual([{ id: '1', name: 'My WS' }]);
  });

  it('getSpaces requires a teamId and passes it', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ spaces: [{ id: 's1', name: 'Eng' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getSpaces()).toEqual([]);
    expect(getResource).not.toHaveBeenCalled();

    await ds.getSpaces('9');
    expect(getResource).toHaveBeenCalledWith('spaces', { teamId: '9' });
  });

  it('getFolders requires a spaceId and passes it', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ folders: [{ id: 'f1', name: 'Q1' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getFolders()).toEqual([]);
    await ds.getFolders('s1');
    expect(getResource).toHaveBeenCalledWith('folders', { spaceId: 's1' });
  });

  it('getLists passes spaceId and/or folderId', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ lists: [{ id: 'l1', name: 'Backlog' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getLists()).toEqual([]);

    await ds.getLists('s1');
    expect(getResource).toHaveBeenCalledWith('lists', { spaceId: 's1' });

    await ds.getLists('s1', 'f1');
    expect(getResource).toHaveBeenCalledWith('lists', { spaceId: 's1', folderId: 'f1' });

    await ds.getLists(undefined, 'f1');
    expect(getResource).toHaveBeenCalledWith('lists', { folderId: 'f1' });
  });

  it('getMembers passes teamId when present', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ members: [{ id: '10', username: 'Alice' }] });
    (ds as any).getResource = getResource;

    await ds.getMembers('9');
    expect(getResource).toHaveBeenCalledWith('members', { teamId: '9' });

    await ds.getMembers();
    expect(getResource).toHaveBeenCalledWith('members', undefined);
  });

  it('getTaskFields fetches the field catalog', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ fields: ['id', 'name', 'status'] });
    (ds as any).getResource = getResource;

    expect(await ds.getTaskFields()).toEqual(['id', 'name', 'status']);
    expect(getResource).toHaveBeenCalledWith('taskfields');
  });

  it('getDefaultQuery returns tasks with created ordering', () => {
    const ds = makeDS();
    const q = ds.getDefaultQuery({} as any);
    expect(q.queryType).toBe('tasks');
    expect(q.orderBy).toBe('created');
  });
});

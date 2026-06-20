import { DataSource } from './datasource';
import { AsanaQuery } from './types';

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-asana-datasource',
    name: 'Asana',
    jsonData: { baseURL: 'https://app.asana.com/api/1.0' },
  } as any);

describe('Asana DataSource', () => {
  it('filterQuery always runs predefined queries', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'tasks' } as AsanaQuery)).toBe(true);
    expect(ds.filterQuery({ refId: 'A', queryType: 'projects' } as AsanaQuery)).toBe(true);
  });

  it('filterQuery requires a path for raw queries', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw' } as AsanaQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw', rawPath: '   ' } as AsanaQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw', rawPath: '/workspaces' } as AsanaQuery)).toBe(true);
  });

  it('getWorkspaces fetches all workspaces', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ workspaces: [{ gid: '1', name: 'My Org' }] });
    (ds as any).getResource = getResource;

    const workspaces = await ds.getWorkspaces();
    expect(getResource).toHaveBeenCalledWith('workspaces');
    expect(workspaces).toEqual([{ gid: '1', name: 'My Org' }]);
  });

  it('getTeams requires a workspace and passes it', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ teams: [{ gid: 't1', name: 'Eng' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getTeams()).toEqual([]);
    expect(getResource).not.toHaveBeenCalled();

    await ds.getTeams('9');
    expect(getResource).toHaveBeenCalledWith('teams', { workspace: '9' });
  });

  it('getProjects passes workspace and/or team', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ projects: [{ gid: 'p1', name: 'Mobile' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getProjects()).toEqual([]);

    await ds.getProjects('w1');
    expect(getResource).toHaveBeenCalledWith('projects', { workspace: 'w1' });

    await ds.getProjects('w1', 't1');
    expect(getResource).toHaveBeenCalledWith('projects', { workspace: 'w1', team: 't1' });

    await ds.getProjects(undefined, 't1');
    expect(getResource).toHaveBeenCalledWith('projects', { team: 't1' });
  });

  it('getSections requires a project and passes it', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ sections: [{ gid: 's1', name: 'To Do' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getSections()).toEqual([]);
    await ds.getSections('p1');
    expect(getResource).toHaveBeenCalledWith('sections', { project: 'p1' });
  });

  it('getUsers passes the workspace', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ users: [{ gid: '10', name: 'Alice' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getUsers()).toEqual([]);
    await ds.getUsers('9');
    expect(getResource).toHaveBeenCalledWith('users', { workspace: '9' });
  });

  it('getTags passes the workspace', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ tags: [{ gid: 'tg1', name: 'urgent' }] });
    (ds as any).getResource = getResource;

    await ds.getTags('9');
    expect(getResource).toHaveBeenCalledWith('tags', { workspace: '9' });
  });

  it('getTaskFields fetches the field catalog', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ fields: ['gid', 'name', 'assignee'] });
    (ds as any).getResource = getResource;

    expect(await ds.getTaskFields()).toEqual(['gid', 'name', 'assignee']);
    expect(getResource).toHaveBeenCalledWith('taskfields');
  });

  it('getDefaultQuery returns tasks', () => {
    const ds = makeDS();
    const q = ds.getDefaultQuery({} as any);
    expect(q.queryType).toBe('tasks');
  });
});

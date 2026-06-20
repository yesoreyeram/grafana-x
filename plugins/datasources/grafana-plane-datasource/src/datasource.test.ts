import { DataSource } from './datasource';
import { PlaneQuery } from './types';

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-plane-datasource',
    name: 'Plane',
    jsonData: { baseURL: 'https://api.plane.so', authMethod: 'apiKey', workspaceSlug: 'my-team' },
  } as any);

describe('Plane DataSource', () => {
  it('filterQuery always runs predefined queries', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'workitems' } as PlaneQuery)).toBe(true);
    expect(ds.filterQuery({ refId: 'A', queryType: 'projects' } as PlaneQuery)).toBe(true);
    expect(ds.filterQuery({ refId: 'A', queryType: 'members' } as PlaneQuery)).toBe(true);
  });

  it('filterQuery requires a path for raw queries', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw' } as PlaneQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw', rawPath: '   ' } as PlaneQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw', rawPath: '/api/v1/users/me/' } as PlaneQuery)).toBe(true);
  });

  it('getProjects passes the workspace when present', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ projects: [{ id: 'p1', name: 'Apollo' }] });
    (ds as any).getResource = getResource;

    const projects = await ds.getProjects('my-team');
    expect(getResource).toHaveBeenCalledWith('projects', { workspace: 'my-team' });
    expect(projects).toEqual([{ id: 'p1', name: 'Apollo' }]);

    await ds.getProjects();
    expect(getResource).toHaveBeenCalledWith('projects', undefined);
  });

  it('getStates requires a projectId and passes params', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ states: [{ id: 's1', name: 'Todo' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getStates('my-team')).toEqual([]);
    expect(getResource).not.toHaveBeenCalled();

    await ds.getStates('my-team', 'p1');
    expect(getResource).toHaveBeenCalledWith('states', { projectId: 'p1', workspace: 'my-team' });
  });

  it('getLabels requires a projectId and passes params', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ labels: [{ id: 'l1', name: 'bug' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getLabels('my-team')).toEqual([]);
    await ds.getLabels('my-team', 'p1');
    expect(getResource).toHaveBeenCalledWith('labels', { projectId: 'p1', workspace: 'my-team' });
  });

  it('getMembers passes the workspace when present', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ members: [{ id: 'u1', display_name: 'Alice' }] });
    (ds as any).getResource = getResource;

    await ds.getMembers('my-team');
    expect(getResource).toHaveBeenCalledWith('members', { workspace: 'my-team' });

    await ds.getMembers();
    expect(getResource).toHaveBeenCalledWith('members', undefined);
  });

  it('getWorkItemFields fetches the field catalog', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ fields: ['id', 'name', 'state'] });
    (ds as any).getResource = getResource;

    expect(await ds.getWorkItemFields()).toEqual(['id', 'name', 'state']);
    expect(getResource).toHaveBeenCalledWith('workitemfields');
  });

  it('getPriorities fetches the priority list', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ priorities: ['urgent', 'high'] });
    (ds as any).getResource = getResource;

    expect(await ds.getPriorities()).toEqual(['urgent', 'high']);
    expect(getResource).toHaveBeenCalledWith('priorities');
  });

  it('getDefaultQuery returns work items ordered by newest first', () => {
    const ds = makeDS();
    const q = ds.getDefaultQuery({} as any);
    expect(q.queryType).toBe('workitems');
    expect(q.orderBy).toBe('-created_at');
  });
});

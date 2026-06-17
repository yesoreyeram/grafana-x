import { DataSource } from './datasource';
import { LinearQuery } from './types';

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-linear-datasource',
    name: 'Linear',
    jsonData: { baseURL: 'https://api.linear.app/graphql', authMethod: 'apiKey' },
  } as any);

describe('Linear DataSource', () => {
  it('filterQuery always runs predefined queries', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'issues' } as LinearQuery)).toBe(true);
    expect(ds.filterQuery({ refId: 'A', queryType: 'projects' } as LinearQuery)).toBe(true);
  });

  it('filterQuery requires a document for raw queries', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw' } as LinearQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw', rawQuery: '   ' } as LinearQuery)).toBe(false);
    expect(ds.filterQuery({ refId: 'A', queryType: 'raw', rawQuery: 'query{viewer{id}}' } as LinearQuery)).toBe(true);
  });

  it('getTeams fetches all teams', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      teams: [{ id: '1', key: 'ENG', name: 'Engineering' }],
    });
    (ds as any).getResource = getResource;

    const teams = await ds.getTeams();
    expect(getResource).toHaveBeenCalledWith('teams');
    expect(teams).toEqual([{ id: '1', key: 'ENG', name: 'Engineering' }]);
  });

  it('getStates passes teamId when present', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ states: [{ name: 'Todo' }] });
    (ds as any).getResource = getResource;

    await ds.getStates('team-1');
    expect(getResource).toHaveBeenCalledWith('states', { teamId: 'team-1' });

    await ds.getStates();
    expect(getResource).toHaveBeenCalledWith('states', undefined);
  });

  it('getLabels passes teamId when present', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ labels: [{ name: 'bug' }] });
    (ds as any).getResource = getResource;

    const labels = await ds.getLabels('team-1');
    expect(getResource).toHaveBeenCalledWith('labels', { teamId: 'team-1' });
    expect(labels).toEqual([{ name: 'bug' }]);
  });

  it('getProjects / getUsers fetch lists', async () => {
    const ds = makeDS();
    const getResource = jest
      .fn()
      .mockResolvedValueOnce({ projects: [{ id: 'p1', name: 'Mobile' }] })
      .mockResolvedValueOnce({ users: [{ name: 'Alice', email: 'a@b.com' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getProjects()).toEqual([{ id: 'p1', name: 'Mobile' }]);
    expect(getResource).toHaveBeenCalledWith('projects');
    expect(await ds.getUsers()).toEqual([{ name: 'Alice', email: 'a@b.com' }]);
    expect(getResource).toHaveBeenCalledWith('users');
  });

  it('getIssueFields fetches the field catalog', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ fields: ['identifier', 'title', 'state'] });
    (ds as any).getResource = getResource;

    expect(await ds.getIssueFields()).toEqual(['identifier', 'title', 'state']);
    expect(getResource).toHaveBeenCalledWith('issuefields');
  });

  it('getDefaultQuery returns issues with createdAt ordering', () => {
    const ds = makeDS();
    const q = ds.getDefaultQuery({} as any);
    expect(q.queryType).toBe('issues');
    expect(q.orderBy).toBe('createdAt');
  });
});

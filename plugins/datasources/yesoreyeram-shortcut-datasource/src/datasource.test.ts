import { DataSource } from './datasource';
import { ShortcutQuery } from './types';

// Mock @grafana/runtime so applyTemplateVariables can resolve template variables
// without a live Grafana runtime. The fake template service substitutes
// $var / ${var} tokens from the provided scopedVars.
jest.mock('@grafana/runtime', () => {
  const actual = jest.requireActual('@grafana/runtime');
  return {
    ...actual,
    getTemplateSrv: () => ({
      replace: (value: string, scopedVars: Record<string, { value: unknown }> = {}) => {
        if (!value) {
          return value;
        }
        return value.replace(/\$\{?(\w+)\}?/g, (match, name) => {
          const v = scopedVars[name];
          return v != null ? String(v.value) : match;
        });
      },
    }),
  };
});

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-shortcut-datasource',
    name: 'Shortcut',
    jsonData: { baseURL: 'https://api.app.shortcut.com' },
  } as any);

describe('Shortcut DataSource', () => {
  it('getDefaultQuery returns stories with sensible defaults', () => {
    const ds = makeDS();
    const q = ds.getDefaultQuery({} as any);
    expect(q.queryType).toBe('stories');
    expect(q.dateMode).toBe('any');
    expect(q.archived).toBe('any');
    expect(q.detail).toBe('full');
  });

  it('filterQuery always runs', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'stories' } as ShortcutQuery)).toBe(true);
    expect(ds.filterQuery({ refId: 'A', queryType: 'count' } as ShortcutQuery)).toBe(true);
  });

  it('getProjects/getEpics/getIterations/getLabels call the matching resources', async () => {
    const ds = makeDS();
    const getResource = jest
      .fn()
      .mockResolvedValueOnce({ projects: [{ id: 1, name: 'Backend' }] })
      .mockResolvedValueOnce({ epics: [{ id: 5, name: 'Auth' }] })
      .mockResolvedValueOnce({ iterations: [{ id: 3, name: 'Sprint 12' }] })
      .mockResolvedValueOnce({ labels: [{ id: 1, name: 'bug' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getProjects()).toEqual([{ id: 1, name: 'Backend' }]);
    expect(await ds.getEpics()).toEqual([{ id: 5, name: 'Auth' }]);
    expect(await ds.getIterations()).toEqual([{ id: 3, name: 'Sprint 12' }]);
    expect(await ds.getLabels()).toEqual([{ id: 1, name: 'bug' }]);
    expect(getResource).toHaveBeenNthCalledWith(1, 'projects');
    expect(getResource).toHaveBeenNthCalledWith(2, 'epics');
    expect(getResource).toHaveBeenNthCalledWith(3, 'iterations');
    expect(getResource).toHaveBeenNthCalledWith(4, 'labels');
  });

  it('getMembers and getTeams return nested name/mention_name', async () => {
    const ds = makeDS();
    const getResource = jest
      .fn()
      .mockResolvedValueOnce({ members: [{ id: 'u1', name: 'Alice', mention_name: 'alice' }] })
      .mockResolvedValueOnce({ teams: [{ id: 't1', name: 'Engineering' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getMembers()).toEqual([{ id: 'u1', name: 'Alice', mention_name: 'alice' }]);
    expect(await ds.getTeams()).toEqual([{ id: 't1', name: 'Engineering' }]);
    expect(getResource).toHaveBeenNthCalledWith(1, 'members');
    expect(getResource).toHaveBeenNthCalledWith(2, 'teams');
  });

  it('getStoryFields and getStoryTypes fetch the catalogs', async () => {
    const ds = makeDS();
    const getResource = jest
      .fn()
      .mockResolvedValueOnce({ fields: ['id', 'name', 'owner_ids'] })
      .mockResolvedValueOnce({ storyTypes: ['feature', 'bug', 'chore'] });
    (ds as any).getResource = getResource;

    expect(await ds.getStoryFields()).toEqual(['id', 'name', 'owner_ids']);
    expect(await ds.getStoryTypes()).toEqual(['feature', 'bug', 'chore']);
    expect(getResource).toHaveBeenNthCalledWith(1, 'storyfields');
    expect(getResource).toHaveBeenNthCalledWith(2, 'storytypes');
  });

  it('applyTemplateVariables interpolates scalar fields and lists', () => {
    const ds = makeDS();
    const replaced = ds.applyTemplateVariables(
      {
        refId: 'A',
        queryType: 'stories',
        query: '$text',
        epic: '$epic',
        projects: ['$proj'],
        owners: ['alice'],
        fields: ['id', 'name'],
      } as ShortcutQuery,
      {
        text: { text: 'login', value: 'login' },
        epic: { text: 'Auth', value: 'Auth' },
        proj: { text: 'Backend', value: 'Backend' },
      } as any
    );

    expect(replaced.query).toBe('login');
    expect(replaced.epic).toBe('Auth');
    expect(replaced.projects).toEqual(['Backend']);
    expect(replaced.owners).toEqual(['alice']);
    expect(replaced.fields).toEqual(['id', 'name']);
  });
});

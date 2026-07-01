import { DataSource } from './datasource';
import { TodoistQuery } from './types';

jest.mock('@grafana/runtime', () => ({
  ...(jest.requireActual('@grafana/runtime') as object),
  getTemplateSrv: () => ({
    replace: (v?: string) => (v ? v.replace('$proj', 'p-resolved') : v),
  }),
}));

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-todoist-datasource',
    name: 'Todoist',
    jsonData: {},
  } as any);

describe('Todoist DataSource', () => {
  it('filterQuery always returns true', () => {
    const ds = makeDS();
    expect(ds.filterQuery({ refId: 'A', queryType: 'tasks' } as TodoistQuery)).toBe(true);
    expect(ds.filterQuery({ refId: 'A', queryType: 'count' } as TodoistQuery)).toBe(true);
  });

  it('getProjects fetches projects', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ projects: [{ id: '1', name: 'My Project' }] });
    (ds as any).getResource = getResource;

    const projects = await ds.getProjects();
    expect(getResource).toHaveBeenCalledWith('projects');
    expect(projects).toEqual([{ id: '1', name: 'My Project' }]);
  });

  it('getSections requires a project and passes it', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ sections: [{ id: 's1', name: 'To Do' }] });
    (ds as any).getResource = getResource;

    expect(await ds.getSections()).toEqual([]);
    expect(getResource).not.toHaveBeenCalled();

    await ds.getSections('p1');
    expect(getResource).toHaveBeenCalledWith('sections', { projectId: 'p1' });
  });

  it('getLabels fetches all labels', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ labels: [{ id: 'l1', name: 'urgent' }] });
    (ds as any).getResource = getResource;

    const labels = await ds.getLabels();
    expect(getResource).toHaveBeenCalledWith('labels');
    expect(labels).toEqual([{ id: 'l1', name: 'urgent' }]);
  });

  it('getDefaultQuery returns tasks', () => {
    const ds = makeDS();
    const q = ds.getDefaultQuery({} as any);
    expect(q.queryType).toBe('tasks');
  });

  it('applyTemplateVariables interpolates scope, filter and lang', () => {
    const ds = makeDS();
    const out = ds.applyTemplateVariables(
      {
        refId: 'A',
        queryType: 'tasks',
        projectId: '$proj',
        sectionId: 's1',
        label: 'urgent',
        parentId: 'parent1',
        filter: 'today',
        lang: 'en',
      } as TodoistQuery,
      {}
    );
    expect(out.projectId).toBe('p-resolved');
    expect(out.sectionId).toBe('s1');
    expect(out.label).toBe('urgent');
    expect(out.parentId).toBe('parent1');
    expect(out.filter).toBe('today');
    expect(out.lang).toBe('en');
  });
});

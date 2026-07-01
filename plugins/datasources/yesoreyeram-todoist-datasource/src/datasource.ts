import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { TodoistQuery, TodoistDataSourceOptions, ProjectInfo, SectionInfo, LabelInfo, DEFAULT_QUERY } from './types';

export class DataSource extends DataSourceWithBackend<TodoistQuery, TodoistDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<TodoistDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<TodoistQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: TodoistQuery, scopedVars: ScopedVars): TodoistQuery {
    const templateSrv = getTemplateSrv();
    const replace = (value?: string) => (value ? templateSrv.replace(value, scopedVars) : value);

    return {
      ...query,
      projectId: replace(query.projectId),
      sectionId: replace(query.sectionId),
      label: replace(query.label),
      parentId: replace(query.parentId),
      filter: replace(query.filter),
      lang: replace(query.lang),
    };
  }

  filterQuery(query: TodoistQuery): boolean {
    return true;
  }

  async getProjects(): Promise<ProjectInfo[]> {
    const res = await this.getResource('projects');
    return (res?.projects ?? []) as ProjectInfo[];
  }

  async getSections(projectId?: string): Promise<SectionInfo[]> {
    if (!projectId) {
      return [];
    }
    const res = await this.getResource('sections', { projectId });
    return (res?.sections ?? []) as SectionInfo[];
  }

  async getLabels(): Promise<LabelInfo[]> {
    const res = await this.getResource('labels');
    return (res?.labels ?? []) as LabelInfo[];
  }
}

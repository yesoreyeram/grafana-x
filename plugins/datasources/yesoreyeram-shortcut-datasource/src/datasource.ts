import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  ShortcutQuery,
  ShortcutDataSourceOptions,
  DEFAULT_QUERY,
  ProjectInfo,
  EpicInfo,
  IterationInfo,
  MemberInfo,
  TeamInfo,
  LabelInfo,
  WorkflowStateInfo,
} from './types';

export class DataSource extends DataSourceWithBackend<ShortcutQuery, ShortcutDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<ShortcutDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<ShortcutQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: ShortcutQuery, scopedVars: ScopedVars): ShortcutQuery {
    const templateSrv = getTemplateSrv();
    const replace = (value?: string) => (value ? templateSrv.replace(value, scopedVars) : value);
    // For multi-value lists, interpolate each entry. A multi-value variable
    // expands to comma-separated tokens (csv format), which we split so the
    // backend receives a flat list.
    const replaceList = (values?: string[]) =>
      values
        ?.flatMap((v) => templateSrv.replace(v, scopedVars, 'csv').split(','))
        .map((v) => v.trim())
        .filter((v) => v.length > 0);

    return {
      ...query,
      query: replace(query.query),
      storyType: query.storyType,
      projects: replaceList(query.projects),
      workflowStates: replaceList(query.workflowStates),
      epic: replace(query.epic),
      iteration: replace(query.iteration),
      labels: replaceList(query.labels),
      owners: replaceList(query.owners),
      teams: replaceList(query.teams),
      createdAfter: replace(query.createdAfter),
      createdBefore: replace(query.createdBefore),
      updatedAfter: replace(query.updatedAfter),
      updatedBefore: replace(query.updatedBefore),
      deadlineAfter: replace(query.deadlineAfter),
      deadlineBefore: replace(query.deadlineBefore),
      fields: replaceList(query.fields),
    };
  }

  filterQuery(_query: ShortcutQuery): boolean {
    // Predefined queries are always runnable; the backend validates and returns
    // a clear error otherwise.
    return true;
  }

  async getProjects(): Promise<ProjectInfo[]> {
    const res = await this.getResource('projects');
    return (res?.projects ?? []) as ProjectInfo[];
  }

  async getEpics(): Promise<EpicInfo[]> {
    const res = await this.getResource('epics');
    return (res?.epics ?? []) as EpicInfo[];
  }

  async getIterations(): Promise<IterationInfo[]> {
    const res = await this.getResource('iterations');
    return (res?.iterations ?? []) as IterationInfo[];
  }

  async getMembers(): Promise<MemberInfo[]> {
    const res = await this.getResource('members');
    return (res?.members ?? []) as MemberInfo[];
  }

  async getTeams(): Promise<TeamInfo[]> {
    const res = await this.getResource('teams');
    return (res?.teams ?? []) as TeamInfo[];
  }

  async getLabels(): Promise<LabelInfo[]> {
    const res = await this.getResource('labels');
    return (res?.labels ?? []) as LabelInfo[];
  }

  async getWorkflows(): Promise<WorkflowStateInfo[]> {
    const res = await this.getResource('workflows');
    return (res?.workflows ?? []) as WorkflowStateInfo[];
  }

  async getStoryFields(): Promise<string[]> {
    const res = await this.getResource('storyfields');
    return (res?.fields ?? []) as string[];
  }

  async getStoryTypes(): Promise<string[]> {
    const res = await this.getResource('storytypes');
    return (res?.storyTypes ?? []) as string[];
  }
}

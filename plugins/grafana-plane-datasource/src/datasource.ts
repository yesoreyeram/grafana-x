import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  PlaneQuery,
  PlaneDataSourceOptions,
  DEFAULT_QUERY,
  ProjectInfo,
  StateInfo,
  LabelInfo,
  MemberInfo,
} from './types';

export class DataSource extends DataSourceWithBackend<PlaneQuery, PlaneDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<PlaneDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<PlaneQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: PlaneQuery, scopedVars: ScopedVars): PlaneQuery {
    const templateSrv = getTemplateSrv();
    const replace = (value?: string) => (value ? templateSrv.replace(value, scopedVars) : value);
    // For multi-value lists, interpolate each entry. A multi-value variable
    // expands to comma-separated tokens (csv format), which we then split so the
    // backend receives a flat list.
    const replaceList = (values?: string[]) =>
      values
        ?.flatMap((v) => templateSrv.replace(v, scopedVars, 'csv').split(','))
        .map((v) => v.trim())
        .filter((v) => v.length > 0);

    return {
      ...query,
      workspaceSlug: replace(query.workspaceSlug),
      projectId: replace(query.projectId),
      priorities: replaceList(query.priorities),
      states: replaceList(query.states),
      assignees: replaceList(query.assignees),
      labels: replaceList(query.labels),
      createdAfter: replace(query.createdAfter),
      createdBefore: replace(query.createdBefore),
      updatedAfter: replace(query.updatedAfter),
      updatedBefore: replace(query.updatedBefore),
      orderBy: replace(query.orderBy),
      rawPath: replace(query.rawPath),
      rawRoot: replace(query.rawRoot),
    };
  }

  filterQuery(query: PlaneQuery): boolean {
    // Raw queries need a path; predefined queries are always runnable (the
    // backend validates the required scope and returns a clear error otherwise).
    if (query.queryType === 'raw') {
      return !!query.rawPath && query.rawPath.trim().length > 0;
    }
    return true;
  }

  /** Fetch the projects in a workspace via the backend resource handler. */
  async getProjects(workspace?: string): Promise<ProjectInfo[]> {
    const res = await this.getResource('projects', workspace ? { workspace } : undefined);
    return (res?.projects ?? []) as ProjectInfo[];
  }

  /** Fetch the states in a project via the backend resource handler. */
  async getStates(workspace?: string, projectId?: string): Promise<StateInfo[]> {
    if (!projectId) {
      return [];
    }
    const params: Record<string, string> = { projectId };
    if (workspace) {
      params.workspace = workspace;
    }
    const res = await this.getResource('states', params);
    return (res?.states ?? []) as StateInfo[];
  }

  /** Fetch the labels in a project via the backend resource handler. */
  async getLabels(workspace?: string, projectId?: string): Promise<LabelInfo[]> {
    if (!projectId) {
      return [];
    }
    const params: Record<string, string> = { projectId };
    if (workspace) {
      params.workspace = workspace;
    }
    const res = await this.getResource('labels', params);
    return (res?.labels ?? []) as LabelInfo[];
  }

  /** Fetch the workspace members via the backend resource handler. */
  async getMembers(workspace?: string): Promise<MemberInfo[]> {
    const res = await this.getResource('members', workspace ? { workspace } : undefined);
    return (res?.members ?? []) as MemberInfo[];
  }

  /** Fetch the selectable work item field names via the backend resource handler. */
  async getWorkItemFields(): Promise<string[]> {
    const res = await this.getResource('workitemfields');
    return (res?.fields ?? []) as string[];
  }

  /** Fetch the known work item priorities via the backend resource handler. */
  async getPriorities(): Promise<string[]> {
    const res = await this.getResource('priorities');
    return (res?.priorities ?? []) as string[];
  }
}

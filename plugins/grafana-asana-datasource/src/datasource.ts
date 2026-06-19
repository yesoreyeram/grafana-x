import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { AsanaQuery, AsanaDataSourceOptions, AsanaResource, DEFAULT_QUERY } from './types';

export class DataSource extends DataSourceWithBackend<AsanaQuery, AsanaDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<AsanaDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<AsanaQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: AsanaQuery, scopedVars: ScopedVars): AsanaQuery {
    const templateSrv = getTemplateSrv();
    const replace = (value?: string) => (value ? templateSrv.replace(value, scopedVars) : value);

    return {
      ...query,
      workspace: replace(query.workspace),
      team: replace(query.team),
      project: replace(query.project),
      section: replace(query.section),
      assignee: replace(query.assignee),
      modifiedSince: replace(query.modifiedSince),
      rawPath: replace(query.rawPath),
      rawRoot: replace(query.rawRoot),
    };
  }

  filterQuery(query: AsanaQuery): boolean {
    // Raw queries need a path; predefined queries are always runnable (the
    // backend validates the required scope and returns a clear error otherwise).
    if (query.queryType === 'raw') {
      return !!query.rawPath && query.rawPath.trim().length > 0;
    }
    return true;
  }

  /** Fetch the visible workspaces/organizations via the backend resource handler. */
  async getWorkspaces(): Promise<AsanaResource[]> {
    const res = await this.getResource('workspaces');
    return (res?.workspaces ?? []) as AsanaResource[];
  }

  /** Fetch the teams in a workspace (organizations only). */
  async getTeams(workspace?: string): Promise<AsanaResource[]> {
    if (!workspace) {
      return [];
    }
    const res = await this.getResource('teams', { workspace });
    return (res?.teams ?? []) as AsanaResource[];
  }

  /** Fetch the projects in a workspace and/or team. */
  async getProjects(workspace?: string, team?: string): Promise<AsanaResource[]> {
    if (!workspace && !team) {
      return [];
    }
    const params: Record<string, string> = {};
    if (workspace) {
      params.workspace = workspace;
    }
    if (team) {
      params.team = team;
    }
    const res = await this.getResource('projects', params);
    return (res?.projects ?? []) as AsanaResource[];
  }

  /** Fetch the sections in a project. */
  async getSections(project?: string): Promise<AsanaResource[]> {
    if (!project) {
      return [];
    }
    const res = await this.getResource('sections', { project });
    return (res?.sections ?? []) as AsanaResource[];
  }

  /** Fetch the users in a workspace, used for the assignee picker. */
  async getUsers(workspace?: string): Promise<AsanaResource[]> {
    if (!workspace) {
      return [];
    }
    const res = await this.getResource('users', { workspace });
    return (res?.users ?? []) as AsanaResource[];
  }

  /** Fetch the tags in a workspace. */
  async getTags(workspace?: string): Promise<AsanaResource[]> {
    if (!workspace) {
      return [];
    }
    const res = await this.getResource('tags', { workspace });
    return (res?.tags ?? []) as AsanaResource[];
  }

  /** Fetch the selectable task field names via the backend resource handler. */
  async getTaskFields(): Promise<string[]> {
    const res = await this.getResource('taskfields');
    return (res?.fields ?? []) as string[];
  }
}

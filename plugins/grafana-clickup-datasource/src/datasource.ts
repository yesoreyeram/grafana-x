import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  ClickUpQuery,
  ClickUpDataSourceOptions,
  DEFAULT_QUERY,
  TeamInfo,
  SpaceInfo,
  FolderInfo,
  ClickUpListInfo,
  MemberInfo,
} from './types';

export class DataSource extends DataSourceWithBackend<ClickUpQuery, ClickUpDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<ClickUpDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<ClickUpQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: ClickUpQuery, scopedVars: ScopedVars): ClickUpQuery {
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
      teamId: replace(query.teamId),
      spaceId: replace(query.spaceId),
      folderId: replace(query.folderId),
      listId: replace(query.listId),
      statuses: replaceList(query.statuses),
      assignees: replaceList(query.assignees),
      tags: replaceList(query.tags),
      createdAfter: replace(query.createdAfter),
      createdBefore: replace(query.createdBefore),
      updatedAfter: replace(query.updatedAfter),
      updatedBefore: replace(query.updatedBefore),
      dueAfter: replace(query.dueAfter),
      dueBefore: replace(query.dueBefore),
      rawPath: replace(query.rawPath),
      rawRoot: replace(query.rawRoot),
    };
  }

  filterQuery(query: ClickUpQuery): boolean {
    // Raw queries need a path; predefined queries are always runnable (the
    // backend validates the required scope and returns a clear error otherwise).
    if (query.queryType === 'raw') {
      return !!query.rawPath && query.rawPath.trim().length > 0;
    }
    return true;
  }

  /** Fetch the authorized workspaces via the backend resource handler. */
  async getTeams(): Promise<TeamInfo[]> {
    const res = await this.getResource('teams');
    return (res?.teams ?? []) as TeamInfo[];
  }

  /** Fetch the spaces in a workspace via the backend resource handler. */
  async getSpaces(teamId?: string): Promise<SpaceInfo[]> {
    if (!teamId) {
      return [];
    }
    const res = await this.getResource('spaces', { teamId });
    return (res?.spaces ?? []) as SpaceInfo[];
  }

  /** Fetch the folders in a space via the backend resource handler. */
  async getFolders(spaceId?: string): Promise<FolderInfo[]> {
    if (!spaceId) {
      return [];
    }
    const res = await this.getResource('folders', { spaceId });
    return (res?.folders ?? []) as FolderInfo[];
  }

  /** Fetch the lists in a folder (or folderless lists in a space). */
  async getLists(spaceId?: string, folderId?: string): Promise<ClickUpListInfo[]> {
    if (!spaceId && !folderId) {
      return [];
    }
    const params: Record<string, string> = {};
    if (spaceId) {
      params.spaceId = spaceId;
    }
    if (folderId) {
      params.folderId = folderId;
    }
    const res = await this.getResource('lists', params);
    return (res?.lists ?? []) as ClickUpListInfo[];
  }

  /** Fetch the workspace members via the backend resource handler. */
  async getMembers(teamId?: string): Promise<MemberInfo[]> {
    const res = await this.getResource('members', teamId ? { teamId } : undefined);
    return (res?.members ?? []) as MemberInfo[];
  }

  /** Fetch the selectable task field names via the backend resource handler. */
  async getTaskFields(): Promise<string[]> {
    const res = await this.getResource('taskfields');
    return (res?.fields ?? []) as string[];
  }
}

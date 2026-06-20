import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  MondayQuery,
  MondayDataSourceOptions,
  DEFAULT_QUERY,
  BoardInfo,
  GroupInfo,
  ColumnInfo,
  WorkspaceInfo,
} from './types';

export class DataSource extends DataSourceWithBackend<MondayQuery, MondayDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<MondayDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<MondayQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: MondayQuery, scopedVars: ScopedVars): MondayQuery {
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
      boardIds: replaceList(query.boardIds),
      groupIds: replaceList(query.groupIds),
      workspaceIds: replaceList(query.workspaceIds),
      columnIds: replaceList(query.columnIds),
      searchQuery: replace(query.searchQuery),
      orderBy: replace(query.orderBy),
      rawQuery: replace(query.rawQuery),
      rawVariables: replace(query.rawVariables),
    };
  }

  filterQuery(query: MondayQuery): boolean {
    // Raw queries need a document.
    if (query.queryType === 'raw') {
      return !!query.rawQuery && query.rawQuery.trim().length > 0;
    }
    // Items and groups require at least one board.
    if (query.queryType === 'items' || query.queryType === 'groups') {
      return !!query.boardIds && query.boardIds.length > 0;
    }
    return true;
  }

  /** Fetch the account boards via the backend resource handler. */
  async getBoards(): Promise<BoardInfo[]> {
    const res = await this.getResource('boards');
    return (res?.boards ?? []) as BoardInfo[];
  }

  /** Fetch groups for the given boards via the backend resource handler. */
  async getGroups(boardIds?: string[]): Promise<GroupInfo[]> {
    const res = await this.getResource('groups', this.boardParams(boardIds));
    return (res?.groups ?? []) as GroupInfo[];
  }

  /** Fetch columns for the given boards via the backend resource handler. */
  async getColumns(boardIds?: string[]): Promise<ColumnInfo[]> {
    const res = await this.getResource('columns', this.boardParams(boardIds));
    return (res?.columns ?? []) as ColumnInfo[];
  }

  /** Fetch the account workspaces via the backend resource handler. */
  async getWorkspaces(): Promise<WorkspaceInfo[]> {
    const res = await this.getResource('workspaces');
    return (res?.workspaces ?? []) as WorkspaceInfo[];
  }

  /** Build repeated boardId query params for a resource call. */
  private boardParams(boardIds?: string[]): Record<string, string[]> | undefined {
    const ids = (boardIds ?? []).filter((id) => id && id.trim().length > 0);
    return ids.length > 0 ? { boardId: ids } : undefined;
  }
}

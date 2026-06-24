import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { TrelloQuery, TrelloDataSourceOptions, DEFAULT_QUERY, BoardInfo, ListInfo, MemberInfo, LabelInfo } from './types';

export class DataSource extends DataSourceWithBackend<TrelloQuery, TrelloDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<TrelloDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<TrelloQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: TrelloQuery, scopedVars: ScopedVars): TrelloQuery {
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
      boardId: replace(query.boardId),
      listId: replace(query.listId),
      memberIds: replaceList(query.memberIds),
      labelIds: replaceList(query.labelIds),
      fields: replaceList(query.fields),
      createdAfter: replace(query.createdAfter),
      createdBefore: replace(query.createdBefore),
    };
  }

  filterQuery(query: TrelloQuery): boolean {
    if (query.queryType === 'cards' || query.queryType === 'count') {
      return !!query.boardId;
    }
    return true;
  }

  async getBoards(): Promise<BoardInfo[]> {
    const res = await this.getResource('boards');
    return (res?.boards ?? []) as BoardInfo[];
  }

  async getLists(boardId: string): Promise<ListInfo[]> {
    if (!boardId) {
      return [];
    }
    const res = await this.getResource('lists', { boardId });
    return (res?.lists ?? []) as ListInfo[];
  }

  async getMembers(boardId: string): Promise<MemberInfo[]> {
    if (!boardId) {
      return [];
    }
    const res = await this.getResource('members', { boardId });
    return (res?.members ?? []) as MemberInfo[];
  }

  async getLabels(boardId: string): Promise<LabelInfo[]> {
    if (!boardId) {
      return [];
    }
    const res = await this.getResource('labels', { boardId });
    return (res?.labels ?? []) as LabelInfo[];
  }
}

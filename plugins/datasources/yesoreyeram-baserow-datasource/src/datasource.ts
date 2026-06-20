import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  BaserowQuery,
  BaserowDataSourceOptions,
  BaserowAuthMode,
  DEFAULT_QUERY,
  TableInfo,
  FieldInfo,
  ViewInfo,
  DatabaseInfo,
} from './types';
import { interpolateFilterTree, parseFilterTree, stringifyFilterTree } from './filter';

export class DataSource extends DataSourceWithBackend<BaserowQuery, BaserowDataSourceOptions> {
  /** Database id from the data source config, used to list tables. */
  databaseId: string;
  /** Authentication mode (token or password). */
  authMode: BaserowAuthMode;

  constructor(instanceSettings: DataSourceInstanceSettings<BaserowDataSourceOptions>) {
    super(instanceSettings);
    this.databaseId = instanceSettings.jsonData?.databaseId ?? '';
    this.authMode = instanceSettings.jsonData?.authMode ?? 'token';
  }

  getDefaultQuery(_app: CoreApp): Partial<BaserowQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: BaserowQuery, scopedVars: ScopedVars): BaserowQuery {
    const templateSrv = getTemplateSrv();

    // Interpolate the filter values inside the structured tree. The backend
    // builds the Baserow `filters` clause from this tree, so values must be
    // concrete here. For list operators use the `csv` format so a multi-value
    // variable expands to comma-separated tokens; otherwise use the default
    // single-value formatting.
    let filterTree = query.filterTree;
    if (filterTree) {
      const tree = interpolateFilterTree(parseFilterTree(filterTree), (value, asList) =>
        templateSrv.replace(value, scopedVars, asList ? 'csv' : undefined)
      );
      filterTree = stringifyFilterTree(tree);
    }

    return {
      ...query,
      tableId: query.tableId ? templateSrv.replace(query.tableId, scopedVars) : query.tableId,
      viewId: query.viewId ? templateSrv.replace(query.viewId, scopedVars) : query.viewId,
      filterTree,
      sort: query.sort ? templateSrv.replace(query.sort, scopedVars) : query.sort,
      fields: query.fields ? templateSrv.replace(query.fields, scopedVars) : query.fields,
    };
  }

  filterQuery(query: BaserowQuery): boolean {
    // Do not execute incomplete queries.
    return !!query.tableId;
  }

  /**
   * Fetch tables via the backend resource handler. When a databaseId is provided
   * that database is listed, otherwise the configured database is used.
   */
  async getTables(databaseId?: string): Promise<TableInfo[]> {
    const params = databaseId ? { databaseId } : undefined;
    const res = await this.getResource('tables', params);
    return (res?.tables ?? []) as TableInfo[];
  }

  /** Fetch the fields of a table via the backend resource handler. */
  async getFields(tableId: string): Promise<FieldInfo[]> {
    if (!tableId) {
      return [];
    }
    const res = await this.getResource('fields', { tableId });
    return (res?.fields ?? []) as FieldInfo[];
  }

  /**
   * Fetch all accessible databases via the backend resource handler. Only
   * available in the password (JWT) auth mode.
   */
  async getDatabases(): Promise<DatabaseInfo[]> {
    const res = await this.getResource('databases');
    return (res?.databases ?? []) as DatabaseInfo[];
  }

  /** Fetch the views of a table via the backend resource handler. */
  async getViews(tableId: string): Promise<ViewInfo[]> {
    if (!tableId) {
      return [];
    }
    const res = await this.getResource('views', { tableId });
    return (res?.views ?? []) as ViewInfo[];
  }
}

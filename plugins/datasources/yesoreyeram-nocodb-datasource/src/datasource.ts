import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { NocoDBQuery, NocoDBDataSourceOptions, DEFAULT_QUERY, TableInfo, FieldInfo, ViewInfo } from './types';
import { interpolateFilterTree, parseFilterTree, stringifyFilterTree } from './filter';

export class DataSource extends DataSourceWithBackend<NocoDBQuery, NocoDBDataSourceOptions> {
  /** Default base id from the data source config, used to list tables. */
  baseId: string;

  constructor(instanceSettings: DataSourceInstanceSettings<NocoDBDataSourceOptions>) {
    super(instanceSettings);
    this.baseId = instanceSettings.jsonData?.baseId ?? '';
  }

  getDefaultQuery(_app: CoreApp): Partial<NocoDBQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: NocoDBQuery, scopedVars: ScopedVars): NocoDBQuery {
    const templateSrv = getTemplateSrv();

    // Interpolate the filter values inside the structured tree. The backend
    // builds the where clause from this tree, so values must be concrete here.
    // For list operators (in/anyof/...) use the `csv` format so a multi-value
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
      baseId: query.baseId ? templateSrv.replace(query.baseId, scopedVars) : query.baseId,
      viewId: query.viewId ? templateSrv.replace(query.viewId, scopedVars) : query.viewId,
      where: query.where ? templateSrv.replace(query.where, scopedVars) : query.where,
      filterTree,
      sort: query.sort ? templateSrv.replace(query.sort, scopedVars) : query.sort,
      fields: query.fields ? templateSrv.replace(query.fields, scopedVars) : query.fields,
    };
  }

  filterQuery(query: NocoDBQuery): boolean {
    // Do not execute incomplete queries.
    return !!query.tableId;
  }

  /**
   * Fetch tables via the backend resource handler. When a baseId is provided
   * only that base is listed, otherwise tables across all accessible bases are
   * returned.
   */
  async getTables(baseId?: string): Promise<TableInfo[]> {
    const params = baseId ? { baseId } : undefined;
    const res = await this.getResource('tables', params);
    return (res?.tables ?? []) as TableInfo[];
  }

  /** Fetch the columns/fields of a table via the backend resource handler. */
  async getFields(tableId: string): Promise<FieldInfo[]> {
    if (!tableId) {
      return [];
    }
    const res = await this.getResource('fields', { tableId });
    return (res?.fields ?? []) as FieldInfo[];
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

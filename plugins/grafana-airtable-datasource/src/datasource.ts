import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  AirtableQuery,
  AirtableDataSourceOptions,
  DEFAULT_QUERY,
  BaseInfo,
  TableInfo,
  FieldInfo,
  ViewInfo,
} from './types';
import { interpolateFilterTree, parseFilterTree, stringifyFilterTree } from './filter';

export class DataSource extends DataSourceWithBackend<AirtableQuery, AirtableDataSourceOptions> {
  /** Default base id from the data source config, used to list tables. */
  baseId: string;

  constructor(instanceSettings: DataSourceInstanceSettings<AirtableDataSourceOptions>) {
    super(instanceSettings);
    this.baseId = instanceSettings.jsonData?.baseId ?? '';
  }

  getDefaultQuery(_app: CoreApp): Partial<AirtableQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: AirtableQuery, scopedVars: ScopedVars): AirtableQuery {
    const templateSrv = getTemplateSrv();

    // Interpolate the filter values inside the structured tree. The backend
    // compiles the formula from this tree, so values must be concrete here.
    let filterTree = query.filterTree;
    if (filterTree) {
      const tree = interpolateFilterTree(parseFilterTree(filterTree), (value) =>
        templateSrv.replace(value, scopedVars)
      );
      filterTree = stringifyFilterTree(tree);
    }

    return {
      ...query,
      baseId: query.baseId ? templateSrv.replace(query.baseId, scopedVars) : query.baseId,
      tableId: query.tableId ? templateSrv.replace(query.tableId, scopedVars) : query.tableId,
      viewId: query.viewId ? templateSrv.replace(query.viewId, scopedVars) : query.viewId,
      filterTree,
      filterByFormula: query.filterByFormula
        ? templateSrv.replace(query.filterByFormula, scopedVars)
        : query.filterByFormula,
      fields: query.fields ? templateSrv.replace(query.fields, scopedVars) : query.fields,
    };
  }

  filterQuery(query: AirtableQuery): boolean {
    // Do not execute incomplete queries.
    return !!query.tableId && !!(query.baseId || this.baseId);
  }

  /** Fetch all bases accessible to the token via the backend resource handler. */
  async getBases(): Promise<BaseInfo[]> {
    const res = await this.getResource('bases');
    return (res?.bases ?? []) as BaseInfo[];
  }

  /**
   * Fetch tables via the backend resource handler. When a baseId is provided that
   * base is listed, otherwise the configured base is used.
   */
  async getTables(baseId?: string): Promise<TableInfo[]> {
    const params = baseId ? { baseId } : undefined;
    const res = await this.getResource('tables', params);
    return (res?.tables ?? []) as TableInfo[];
  }

  /** Fetch the fields of a table via the backend resource handler. */
  async getFields(tableId: string, baseId?: string): Promise<FieldInfo[]> {
    if (!tableId) {
      return [];
    }
    const res = await this.getResource('fields', baseId ? { tableId, baseId } : { tableId });
    return (res?.fields ?? []) as FieldInfo[];
  }

  /** Fetch the views of a table via the backend resource handler. */
  async getViews(tableId: string, baseId?: string): Promise<ViewInfo[]> {
    if (!tableId) {
      return [];
    }
    const res = await this.getResource('views', baseId ? { tableId, baseId } : { tableId });
    return (res?.views ?? []) as ViewInfo[];
  }
}

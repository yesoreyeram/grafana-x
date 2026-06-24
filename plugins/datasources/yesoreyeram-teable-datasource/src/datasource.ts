import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { TeableQuery, TeableDataSourceOptions, DEFAULT_QUERY, TableInfo, FieldInfo } from './types';
import { interpolateFilterTree, parseFilterTree, stringifyFilterTree } from './filter';

export class DataSource extends DataSourceWithBackend<TeableQuery, TeableDataSourceOptions> {
  /** Default base id from the data source config, used to list tables. */
  defaultBaseId: string;

  constructor(instanceSettings: DataSourceInstanceSettings<TeableDataSourceOptions>) {
    super(instanceSettings);
    this.defaultBaseId = instanceSettings.jsonData?.defaultBaseId ?? '';
  }

  getDefaultQuery(_app: CoreApp): Partial<TeableQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: TeableQuery, scopedVars: ScopedVars): TeableQuery {
    const templateSrv = getTemplateSrv();

    // Interpolate the filter values inside the structured tree. The backend
    // compiles the filter object from this tree, so values must be concrete here.
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
      fields: query.fields ? templateSrv.replace(query.fields, scopedVars) : query.fields,
    };
  }

  filterQuery(query: TeableQuery): boolean {
    // A table id is sufficient to run a query; the record/count endpoints are
    // addressed by table id alone (the base id only drives the editor pickers).
    return !!query.tableId;
  }

  /**
   * Fetch tables via the backend resource handler. When a baseId is provided that
   * base is listed, otherwise the configured default base is used.
   */
  async getTables(baseId?: string): Promise<TableInfo[]> {
    const id = baseId || this.defaultBaseId;
    if (!id) {
      return [];
    }
    const res = await this.getResource('tables', { baseId: id });
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
}

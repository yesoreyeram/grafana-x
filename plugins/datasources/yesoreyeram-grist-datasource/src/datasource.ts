import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  GristQuery,
  GristDataSourceOptions,
  DEFAULT_QUERY,
  DocInfo,
  TableInfo,
  FieldInfo,
} from './types';
import { interpolateFilterTree, parseFilterTree, stringifyFilterTree } from './filter';

export class DataSource extends DataSourceWithBackend<GristQuery, GristDataSourceOptions> {
  /** Default doc id from the data source config, used to list tables. */
  docId: string;

  constructor(instanceSettings: DataSourceInstanceSettings<GristDataSourceOptions>) {
    super(instanceSettings);
    this.docId = instanceSettings.jsonData?.docId ?? '';
  }

  getDefaultQuery(_app: CoreApp): Partial<GristQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: GristQuery, scopedVars: ScopedVars): GristQuery {
    const templateSrv = getTemplateSrv();

    let filterTree = query.filterTree;
    if (filterTree) {
      const tree = interpolateFilterTree(parseFilterTree(filterTree), (value) =>
        templateSrv.replace(value, scopedVars)
      );
      filterTree = stringifyFilterTree(tree);
    }

    return {
      ...query,
      docId: query.docId ? templateSrv.replace(query.docId, scopedVars) : query.docId,
      tableId: query.tableId ? templateSrv.replace(query.tableId, scopedVars) : query.tableId,
      filterTree,
      sort: query.sort ? templateSrv.replace(query.sort, scopedVars) : query.sort,
      fields: query.fields ? templateSrv.replace(query.fields, scopedVars) : query.fields,
      sql: query.sql ? templateSrv.replace(query.sql, scopedVars) : query.sql,
    };
  }

  filterQuery(query: GristQuery): boolean {
    if (query.queryType === 'sql') {
      return !!query.sql && query.sql.trim().length > 0;
    }
    return !!query.tableId;
  }

  /** Fetch all docs accessible to the token via the backend resource handler. */
  async getDocs(): Promise<DocInfo[]> {
    const res = await this.getResource('docs');
    return (res?.docs ?? []) as DocInfo[];
  }

  /**
   * Fetch tables via the backend resource handler. When a docId is provided
   * that doc is listed, otherwise the configured doc is used.
   */
  async getTables(docId?: string): Promise<TableInfo[]> {
    const params = docId ? { docId } : undefined;
    const res = await this.getResource('tables', params);
    return (res?.tables ?? []) as TableInfo[];
  }

  /** Fetch the fields of a table via the backend resource handler. */
  async getFields(tableId: string, docId?: string): Promise<FieldInfo[]> {
    if (!tableId) {
      return [];
    }
    const res = await this.getResource('fields', docId ? { tableId, docId } : { tableId });
    return (res?.fields ?? []) as FieldInfo[];
  }
}

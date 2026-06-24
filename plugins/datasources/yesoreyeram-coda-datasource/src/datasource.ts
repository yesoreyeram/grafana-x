import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { CodaQuery, CodaDataSourceOptions, DEFAULT_QUERY, DocInfo, TableInfo, ColumnInfo } from './types';

export class DataSource extends DataSourceWithBackend<CodaQuery, CodaDataSourceOptions> {
  docId: string;

  constructor(instanceSettings: DataSourceInstanceSettings<CodaDataSourceOptions>) {
    super(instanceSettings);
    this.docId = instanceSettings.jsonData?.docId ?? '';
  }

  getDefaultQuery(_app: CoreApp): Partial<CodaQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: CodaQuery, scopedVars: ScopedVars): CodaQuery {
    const templateSrv = getTemplateSrv();
    const interp = (value?: string) => (value ? templateSrv.replace(value, scopedVars) : value);
    return {
      ...query,
      docId: interp(query.docId),
      tableId: interp(query.tableId),
      columns: interp(query.columns),
      filterColumn: interp(query.filterColumn),
      filterValue: interp(query.filterValue),
      query: interp(query.query),
    };
  }

  filterQuery(query: CodaQuery): boolean {
    return !!query.tableId && !!(query.docId || this.docId);
  }

  async getDocs(): Promise<DocInfo[]> {
    const res = await this.getResource('docs');
    return (res?.docs ?? []) as DocInfo[];
  }

  async getTables(docId?: string): Promise<TableInfo[]> {
    const params = docId ? { docId } : undefined;
    const res = await this.getResource('tables', params);
    return (res?.tables ?? []) as TableInfo[];
  }

  async getColumns(tableId: string, docId?: string): Promise<ColumnInfo[]> {
    if (!tableId) {
      return [];
    }
    const res = await this.getResource('columns', docId ? { tableId, docId } : { tableId });
    return (res?.columns ?? []) as ColumnInfo[];
  }
}

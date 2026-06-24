import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { SupabaseQuery, SupabaseDataSourceOptions, DEFAULT_QUERY, TableInfo } from './types';
import { interpolateFilterTree, parseFilterTree, stringifyFilterTree } from './filter';

export class DataSource extends DataSourceWithBackend<SupabaseQuery, SupabaseDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<SupabaseDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<SupabaseQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: SupabaseQuery, scopedVars: ScopedVars): SupabaseQuery {
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
      tableId: query.tableId ? templateSrv.replace(query.tableId, scopedVars) : query.tableId,
      filterTree,
      select: query.select ? templateSrv.replace(query.select, scopedVars) : query.select,
    };
  }

  filterQuery(query: SupabaseQuery): boolean {
    return !!query.tableId;
  }

  /** Fetch tables via the backend resource handler. */
  async getTables(): Promise<TableInfo[]> {
    const res = await this.getResource('tables');
    return (res?.tables ?? []) as TableInfo[];
  }
}

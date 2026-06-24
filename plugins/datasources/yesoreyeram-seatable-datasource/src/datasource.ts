import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { SeaTableQuery, SeaTableDataSourceOptions, DEFAULT_QUERY, TableInfo } from './types';
import { interpolateFilterTree, parseFilterTree, stringifyFilterTree } from './filter';

export class DataSource extends DataSourceWithBackend<SeaTableQuery, SeaTableDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<SeaTableDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<SeaTableQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: SeaTableQuery, scopedVars: ScopedVars): SeaTableQuery {
    const templateSrv = getTemplateSrv();

    // Interpolate the filter values inside the structured tree. The backend
    // compiles the SQL WHERE clause from this tree, so values must be concrete.
    let filterTree = query.filterTree;
    if (filterTree) {
      const tree = interpolateFilterTree(parseFilterTree(filterTree), (value) =>
        templateSrv.replace(value, scopedVars)
      );
      filterTree = stringifyFilterTree(tree);
    }

    return {
      ...query,
      tableName: query.tableName ? templateSrv.replace(query.tableName, scopedVars) : query.tableName,
      viewName: query.viewName ? templateSrv.replace(query.viewName, scopedVars) : query.viewName,
      fields: query.fields ? templateSrv.replace(query.fields, scopedVars) : query.fields,
      sql: query.sql ? templateSrv.replace(query.sql, scopedVars) : query.sql,
      filterTree,
    };
  }

  filterQuery(query: SeaTableQuery): boolean {
    // Do not execute incomplete queries.
    if (query.queryType === 'sql') {
      return !!query.sql && query.sql.trim().length > 0;
    }
    return !!query.tableName;
  }

  /** Fetch the base's tables (with their columns) via the backend resource handler. */
  async getTables(): Promise<TableInfo[]> {
    const res = await this.getResource('tables');
    return (res?.tables ?? []) as TableInfo[];
  }
}

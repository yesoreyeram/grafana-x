import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { NotionQuery, NotionDataSourceOptions, DEFAULT_QUERY, DatabaseInfo, PropertyInfo } from './types';
import { interpolateFilterTree, parseFilterTree, stringifyFilterTree } from './filter';

export class DataSource extends DataSourceWithBackend<NotionQuery, NotionDataSourceOptions> {
  /** Default database id from the data source config. */
  databaseId: string;

  constructor(instanceSettings: DataSourceInstanceSettings<NotionDataSourceOptions>) {
    super(instanceSettings);
    this.databaseId = instanceSettings.jsonData?.databaseId ?? '';
  }

  getDefaultQuery(_app: CoreApp): Partial<NotionQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: NotionQuery, scopedVars: ScopedVars): NotionQuery {
    const templateSrv = getTemplateSrv();

    // Interpolate the filter values inside the structured tree. The backend
    // builds the Notion filter object from this tree, so values must be concrete
    // here. For list operators (in/not_in) use the `csv` format so a multi-value
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
      databaseId: query.databaseId ? templateSrv.replace(query.databaseId, scopedVars) : query.databaseId,
      filterTree,
      sort: query.sort ? templateSrv.replace(query.sort, scopedVars) : query.sort,
      fields: query.fields ? templateSrv.replace(query.fields, scopedVars) : query.fields,
    };
  }

  filterQuery(query: NotionQuery): boolean {
    // Do not execute incomplete queries.
    return !!query.databaseId;
  }

  /** Fetch the databases shared with the integration via the backend resource handler. */
  async getDatabases(): Promise<DatabaseInfo[]> {
    const res = await this.getResource('databases');
    return (res?.databases ?? []) as DatabaseInfo[];
  }

  /** Fetch the properties of a database via the backend resource handler. */
  async getProperties(databaseId: string): Promise<PropertyInfo[]> {
    if (!databaseId) {
      return [];
    }
    const res = await this.getResource('properties', { databaseId });
    return (res?.properties ?? []) as PropertyInfo[];
  }
}

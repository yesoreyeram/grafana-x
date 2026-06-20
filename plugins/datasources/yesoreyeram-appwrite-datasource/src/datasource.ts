import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  AppwriteQuery,
  AppwriteDataSourceOptions,
  DEFAULT_QUERY,
  DatabaseInfo,
  CollectionInfo,
  AttributeInfo,
} from './types';
import { interpolateFilterTree, parseFilterTree, stringifyFilterTree } from './filter';

export class DataSource extends DataSourceWithBackend<AppwriteQuery, AppwriteDataSourceOptions> {
  /** Default database id from the data source config, used to list collections. */
  databaseId: string;

  constructor(instanceSettings: DataSourceInstanceSettings<AppwriteDataSourceOptions>) {
    super(instanceSettings);
    this.databaseId = instanceSettings.jsonData?.databaseId ?? '';
  }

  getDefaultQuery(_app: CoreApp): Partial<AppwriteQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: AppwriteQuery, scopedVars: ScopedVars): AppwriteQuery {
    const templateSrv = getTemplateSrv();

    // Interpolate the filter values inside the structured tree. The backend
    // compiles the query strings from this tree, so values must be concrete here.
    let filterTree = query.filterTree;
    if (filterTree) {
      const tree = interpolateFilterTree(parseFilterTree(filterTree), (value) =>
        templateSrv.replace(value, scopedVars)
      );
      filterTree = stringifyFilterTree(tree);
    }

    return {
      ...query,
      databaseId: query.databaseId ? templateSrv.replace(query.databaseId, scopedVars) : query.databaseId,
      collectionId: query.collectionId ? templateSrv.replace(query.collectionId, scopedVars) : query.collectionId,
      filterTree,
      rawQueries: query.rawQueries ? templateSrv.replace(query.rawQueries, scopedVars) : query.rawQueries,
      attributes: query.attributes ? templateSrv.replace(query.attributes, scopedVars) : query.attributes,
    };
  }

  filterQuery(query: AppwriteQuery): boolean {
    // Do not execute incomplete queries.
    return !!query.collectionId && !!(query.databaseId || this.databaseId);
  }

  /** Fetch all databases in the project via the backend resource handler. */
  async getDatabases(): Promise<DatabaseInfo[]> {
    const res = await this.getResource('databases');
    return (res?.databases ?? []) as DatabaseInfo[];
  }

  /**
   * Fetch collections via the backend resource handler. When a databaseId is
   * provided that database is listed, otherwise the configured database is used.
   */
  async getCollections(databaseId?: string): Promise<CollectionInfo[]> {
    const params = databaseId ? { databaseId } : undefined;
    const res = await this.getResource('collections', params);
    return (res?.collections ?? []) as CollectionInfo[];
  }

  /** Fetch the attributes of a collection via the backend resource handler. */
  async getAttributes(collectionId: string, databaseId?: string): Promise<AttributeInfo[]> {
    if (!collectionId) {
      return [];
    }
    const res = await this.getResource('attributes', databaseId ? { collectionId, databaseId } : { collectionId });
    return (res?.attributes ?? []) as AttributeInfo[];
  }
}

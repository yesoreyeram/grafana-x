import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { PocketBaseQuery, PocketBaseDataSourceOptions, DEFAULT_QUERY, CollectionInfo, FieldInfo } from './types';
import { interpolateFilterTree, parseFilterTree, stringifyFilterTree } from './filter';

export class DataSource extends DataSourceWithBackend<PocketBaseQuery, PocketBaseDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<PocketBaseDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<PocketBaseQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: PocketBaseQuery, scopedVars: ScopedVars): PocketBaseQuery {
    const templateSrv = getTemplateSrv();

    // Interpolate the filter values inside the structured tree. The backend
    // compiles the filter expression from this tree, so values must be concrete
    // here.
    let filterTree = query.filterTree;
    if (filterTree) {
      const tree = interpolateFilterTree(parseFilterTree(filterTree), (value) =>
        templateSrv.replace(value, scopedVars)
      );
      filterTree = stringifyFilterTree(tree);
    }

    return {
      ...query,
      collectionId: query.collectionId ? templateSrv.replace(query.collectionId, scopedVars) : query.collectionId,
      filterTree,
      rawFilter: query.rawFilter ? templateSrv.replace(query.rawFilter, scopedVars) : query.rawFilter,
      fields: query.fields ? templateSrv.replace(query.fields, scopedVars) : query.fields,
    };
  }

  filterQuery(query: PocketBaseQuery): boolean {
    // Do not execute incomplete queries.
    return !!query.collectionId;
  }

  /** Fetch all non-system collections via the backend resource handler. */
  async getCollections(): Promise<CollectionInfo[]> {
    const res = await this.getResource('collections');
    return (res?.collections ?? []) as CollectionInfo[];
  }

  /** Fetch the fields of a collection via the backend resource handler. */
  async getFields(collectionId: string): Promise<FieldInfo[]> {
    if (!collectionId) {
      return [];
    }
    const res = await this.getResource('fields', { collectionId });
    return (res?.fields ?? []) as FieldInfo[];
  }
}

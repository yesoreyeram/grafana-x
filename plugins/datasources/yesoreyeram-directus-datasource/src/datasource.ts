import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  DirectusQuery,
  DirectusDataSourceOptions,
  DEFAULT_QUERY,
  CollectionInfo,
  FieldInfo,
} from './types';
import { interpolateFilterTree, parseFilterTree, stringifyFilterTree } from './filter';

export class DataSource extends DataSourceWithBackend<DirectusQuery, DirectusDataSourceOptions> {
  defaultCollectionId: string;

  constructor(instanceSettings: DataSourceInstanceSettings<DirectusDataSourceOptions>) {
    super(instanceSettings);
    this.defaultCollectionId = instanceSettings.jsonData?.defaultCollectionId ?? '';
  }

  getDefaultQuery(_app: CoreApp): Partial<DirectusQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: DirectusQuery, scopedVars: ScopedVars): DirectusQuery {
    const templateSrv = getTemplateSrv();

    // Interpolate the filter values inside the structured tree. The backend
    // compiles the Directus filter object from this tree, so values must be
    // concrete here. List operators (in/nin/between/nbetween) use the `csv`
    // format so a multi-value variable expands to comma-separated tokens.
    let filterTree = query.filterTree;
    if (filterTree) {
      const tree = interpolateFilterTree(parseFilterTree(filterTree), (value, asList) =>
        templateSrv.replace(value, scopedVars, asList ? 'csv' : undefined)
      );
      filterTree = stringifyFilterTree(tree);
    }

    return {
      ...query,
      collectionId: query.collectionId ? templateSrv.replace(query.collectionId, scopedVars) : query.collectionId,
      filterTree,
      fields: query.fields ? templateSrv.replace(query.fields, scopedVars) : query.fields,
      search: query.search ? templateSrv.replace(query.search, scopedVars) : query.search,
    };
  }

  filterQuery(query: DirectusQuery): boolean {
    return !!query.collectionId;
  }

  async getCollections(): Promise<CollectionInfo[]> {
    const res = await this.getResource('collections');
    return (res?.collections ?? []) as CollectionInfo[];
  }

  async getFields(collectionId: string): Promise<FieldInfo[]> {
    if (!collectionId) {
      return [];
    }
    const res = await this.getResource('fields', { collectionId });
    return (res?.fields ?? []) as FieldInfo[];
  }
}

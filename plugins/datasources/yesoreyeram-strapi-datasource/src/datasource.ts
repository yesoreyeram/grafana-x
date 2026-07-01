import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  StrapiQuery,
  StrapiDataSourceOptions,
  DEFAULT_QUERY,
  ContentTypeInfo,
  FieldInfo,
} from './types';
import { interpolateFilterTree, parseFilterTree, stringifyFilterTree } from './filter';

export class DataSource extends DataSourceWithBackend<StrapiQuery, StrapiDataSourceOptions> {
  defaultContentTypeId: string;

  constructor(instanceSettings: DataSourceInstanceSettings<StrapiDataSourceOptions>) {
    super(instanceSettings);
    this.defaultContentTypeId = instanceSettings.jsonData?.defaultContentTypeId ?? '';
  }

  getDefaultQuery(_app: CoreApp): Partial<StrapiQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: StrapiQuery, scopedVars: ScopedVars): StrapiQuery {
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
      contentTypeId: query.contentTypeId ? templateSrv.replace(query.contentTypeId, scopedVars) : query.contentTypeId,
      filterTree,
      fields: query.fields ? templateSrv.replace(query.fields, scopedVars) : query.fields,
      populate: query.populate ? templateSrv.replace(query.populate, scopedVars) : query.populate,
    };
  }

  filterQuery(query: StrapiQuery): boolean {
    return !!query.contentTypeId;
  }

  async getContentTypes(): Promise<ContentTypeInfo[]> {
    const res = await this.getResource('content-types');
    return (res?.contentTypes ?? []) as ContentTypeInfo[];
  }

  async getFields(contentTypeId: string): Promise<FieldInfo[]> {
    if (!contentTypeId) {
      return [];
    }
    const res = await this.getResource('fields', { contentTypeId });
    return (res?.fields ?? []) as FieldInfo[];
  }
}

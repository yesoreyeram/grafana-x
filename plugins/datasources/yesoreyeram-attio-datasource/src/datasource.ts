import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { AttioQuery, AttioDataSourceOptions, DEFAULT_QUERY, ObjectInfo, AttributeInfo } from './types';
import { interpolateFilterTree, parseFilterTree, stringifyFilterTree } from './filter';

export class DataSource extends DataSourceWithBackend<AttioQuery, AttioDataSourceOptions> {
  defaultObjectId: string;

  constructor(instanceSettings: DataSourceInstanceSettings<AttioDataSourceOptions>) {
    super(instanceSettings);
    this.defaultObjectId = instanceSettings.jsonData?.defaultObjectId ?? '';
  }

  getDefaultQuery(_app: CoreApp): Partial<AttioQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: AttioQuery, scopedVars: ScopedVars): AttioQuery {
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
      objectId: query.objectId ? templateSrv.replace(query.objectId, scopedVars) : query.objectId,
      filterTree,
      fields: query.fields ? templateSrv.replace(query.fields, scopedVars) : query.fields,
    };
  }

  filterQuery(query: AttioQuery): boolean {
    return !!query.objectId;
  }

  async getObjects(): Promise<ObjectInfo[]> {
    const res = await this.getResource('objects');
    return (res?.objects ?? []) as ObjectInfo[];
  }

  async getAttributes(objectId: string): Promise<AttributeInfo[]> {
    if (!objectId) {
      return [];
    }
    const res = await this.getResource('attributes', { objectId });
    return (res?.attributes ?? []) as AttributeInfo[];
  }
}

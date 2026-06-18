import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  HubSpotQuery,
  HubSpotDataSourceOptions,
  DEFAULT_QUERY,
  PropertyInfo,
  PipelineInfo,
  OwnerInfo,
} from './types';

export class DataSource extends DataSourceWithBackend<HubSpotQuery, HubSpotDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<HubSpotDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<HubSpotQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: HubSpotQuery, scopedVars: ScopedVars): HubSpotQuery {
    const templateSrv = getTemplateSrv();
    const replace = (value?: string) => (value ? templateSrv.replace(value, scopedVars) : value);
    const replaceList = (values?: string[]) =>
      values
        ?.flatMap((v) => templateSrv.replace(v, scopedVars, 'csv').split(','))
        .map((v) => v.trim())
        .filter((v) => v.length > 0);

    return {
      ...query,
      sortBy: replace(query.sortBy),
      properties: replaceList(query.properties),
      pipelineId: replace(query.pipelineId),
      stageId: replace(query.stageId),
      createdAfter: replace(query.createdAfter),
      createdBefore: replace(query.createdBefore),
      updatedAfter: replace(query.updatedAfter),
      updatedBefore: replace(query.updatedBefore),
      objectType: replace(query.objectType),
      rawPath: replace(query.rawPath),
      rawBody: replace(query.rawBody),
      rawRoot: replace(query.rawRoot),
      filterGroups: query.filterGroups?.map((fg) => ({
        filters: fg.filters.map((f) => ({
          propertyName: replace(f.propertyName) ?? f.propertyName,
          operator: f.operator,
          value: replace(f.value) ?? f.value,
        })),
      })),
    };
  }

  filterQuery(query: HubSpotQuery): boolean {
    if (query.queryType === 'raw') {
      return !!query.rawPath && query.rawPath.trim().length > 0;
    }
    return true;
  }

  async getProperties(objectType?: string): Promise<PropertyInfo[]> {
    const params: Record<string, string> = {};
    if (objectType) {params.objectType = objectType;}
    const res = await this.getResource('properties', params);
    return (res?.properties ?? []) as PropertyInfo[];
  }

  async getPipelines(objectType?: string): Promise<PipelineInfo[]> {
    const params: Record<string, string> = {};
    if (objectType) {params.objectType = objectType;}
    const res = await this.getResource('pipelines', params);
    return (res?.pipelines ?? []) as PipelineInfo[];
  }

  async getOwners(): Promise<OwnerInfo[]> {
    const res = await this.getResource('owners');
    return (res?.owners ?? []) as OwnerInfo[];
  }

  async getSearchOperators(): Promise<string[]> {
    const res = await this.getResource('search_operators');
    return (res?.operators ?? []) as string[];
  }

  async getObjectTypes(): Promise<string[]> {
    const res = await this.getResource('object_types');
    return (res?.objectTypes ?? []) as string[];
  }
}

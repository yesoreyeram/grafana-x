import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  PipedriveQuery,
  PipedriveDataSourceOptions,
  DEFAULT_QUERY,
  PipelineInfo,
  StageInfo,
  UserInfo,
} from './types';

export class DataSource extends DataSourceWithBackend<PipedriveQuery, PipedriveDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<PipedriveDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<PipedriveQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: PipedriveQuery, scopedVars: ScopedVars): PipedriveQuery {
    const templateSrv = getTemplateSrv();
    const replace = (value?: string) => (value ? templateSrv.replace(value, scopedVars) : value);

    return {
      ...query,
      pipelineId: replace(query.pipelineId),
      stageId: replace(query.stageId),
      userId: replace(query.userId),
      statusFilter: replace(query.statusFilter),
      filterId: replace(query.filterId),
      sortBy: replace(query.sortBy),
      filterGroups: query.filterGroups?.map((fg) => ({
        filters: fg.filters.map((f) => ({
          field: replace(f.field) ?? f.field,
          operator: f.operator,
          value: replace(f.value) ?? f.value,
        })),
      })),
    };
  }

  filterQuery(query: PipedriveQuery): boolean {
    return !!query.queryType;
  }

  async getPipelines(pipelineId?: string): Promise<PipelineInfo[]> {
    const params: Record<string, string> = {};
    if (pipelineId) { params.pipelineId = pipelineId; }
    const res = await this.getResource('pipelines', params);
    return (res?.pipelines ?? []) as PipelineInfo[];
  }

  async getStages(pipelineId?: string): Promise<StageInfo[]> {
    const params: Record<string, string> = {};
    if (pipelineId) { params.pipelineId = pipelineId; }
    const res = await this.getResource('stages', params);
    return (res?.stages ?? []) as StageInfo[];
  }

  async getUsers(): Promise<UserInfo[]> {
    const res = await this.getResource('users');
    return (res?.users ?? []) as UserInfo[];
  }
}

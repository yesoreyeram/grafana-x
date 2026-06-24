import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { IntercomQuery, IntercomDataSourceOptions, DEFAULT_QUERY, AdminInfo, TeamInfo, TagInfo } from './types';
import { interpolateFilters } from './filter';

export class DataSource extends DataSourceWithBackend<IntercomQuery, IntercomDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<IntercomDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<IntercomQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: IntercomQuery, scopedVars: ScopedVars): IntercomQuery {
    const templateSrv = getTemplateSrv();
    const replace = (value?: string) => (value ? templateSrv.replace(value, scopedVars) : value);

    return {
      ...query,
      statusFilter: replace(query.statusFilter),
      role: replace(query.role),
      assigneeId: replace(query.assigneeId),
      teamId: replace(query.teamId),
      tagId: replace(query.tagId),
      searchQuery: replace(query.searchQuery),
      sort: replace(query.sort),
      filters: interpolateFilters(query.filters, (value, asList) =>
        templateSrv.replace(value, scopedVars, asList ? 'csv' : undefined)
      ),
    };
  }

  /** Fetch the workspace admins (teammates) via the backend resource handler. */
  async getAdmins(): Promise<AdminInfo[]> {
    const res = await this.getResource('admins');
    return (res?.admins ?? []) as AdminInfo[];
  }

  /** Fetch the workspace teams via the backend resource handler. */
  async getTeams(): Promise<TeamInfo[]> {
    const res = await this.getResource('teams');
    return (res?.teams ?? []) as TeamInfo[];
  }

  /** Fetch the workspace tags via the backend resource handler. */
  async getTags(): Promise<TagInfo[]> {
    const res = await this.getResource('tags');
    return (res?.tags ?? []) as TagInfo[];
  }
}

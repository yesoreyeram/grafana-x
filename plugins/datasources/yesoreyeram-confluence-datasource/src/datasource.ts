import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { ConfluenceQuery, ConfluenceDataSourceOptions, DEFAULT_QUERY, SpaceInfo } from './types';

export class DataSource extends DataSourceWithBackend<ConfluenceQuery, ConfluenceDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<ConfluenceDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_app: CoreApp): Partial<ConfluenceQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: ConfluenceQuery, scopedVars: ScopedVars): ConfluenceQuery {
    const templateSrv = getTemplateSrv();
    const replace = (value?: string) => (value ? templateSrv.replace(value, scopedVars) : value);

    return {
      ...query,
      spaceId: replace(query.spaceId),
      cql: replace(query.cql),
      sort: replace(query.sort),
      fields: replace(query.fields),
      cursor: replace(query.cursor),
    };
  }

  filterQuery(query: ConfluenceQuery): boolean {
    // Search needs a CQL string; the other types can run without extra input.
    if (query.queryType === 'search') {
      return !!query.cql && query.cql.trim().length > 0;
    }
    return true;
  }

  /** Fetch the spaces visible to the configured credentials. */
  async getSpaces(): Promise<SpaceInfo[]> {
    const res = await this.getResource('spaces');
    return (res?.spaces ?? []) as SpaceInfo[];
  }
}

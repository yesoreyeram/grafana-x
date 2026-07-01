import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type PipedriveQueryType =
  | 'deals' | 'persons' | 'organizations' | 'products' | 'count';

export type PipedriveCountEntity =
  | 'deals' | 'persons' | 'organizations' | 'products';

export type PipedriveAuthMethod = 'apiToken' | 'oauth';

export interface Filter {
  field: string;
  operator: string;
  value: string;
}

export interface FilterGroup {
  filters: Filter[];
}

export interface PipedriveQuery extends DataQuery {
  queryType: PipedriveQueryType;
  pipelineId?: string;
  stageId?: string;
  userId?: string;
  statusFilter?: string;
  filterId?: string;
  countEntity?: PipedriveCountEntity;
  fields?: string[];
  mapCustomFields?: boolean;
  filterGroups?: FilterGroup[];
  sortBy?: string;
  sortDir?: 'ASC' | 'DESC';
  limit?: number;
  start?: number;
  /** When true, hide metadata-style system columns (id/created_at/_*, etc.) from the returned frame. */
  hideSystemFields?: boolean;

}

export const DEFAULT_QUERY: Partial<PipedriveQuery> = {
  queryType: 'deals',
  limit: 100,
  start: 0,
  statusFilter: 'all',
  sortDir: 'DESC',
  countEntity: 'deals',
  mapCustomFields: true,
};

export interface PipedriveDataSourceOptions extends DataSourceJsonData {
  companyDomain?: string;
  authMethod?: PipedriveAuthMethod;
}

export interface PipedriveSecureJsonData {
  apiToken?: string;
  oauthToken?: string;
}

// Resource types returned by the backend
export interface PipelineInfo {
  id: number;
  name: string;
  order_nr: number;
}

export interface StageInfo {
  id: number;
  name: string;
  pipeline_id: number;
  order_nr: number;
}

export interface UserInfo {
  id: number;
  name: string;
  email: string;
}

export const QUERY_TYPE_OPTIONS: Array<{ label: string; value: PipedriveQueryType; description: string }> = [
  { label: 'Deals', value: 'deals', description: 'List CRM deals with pipeline, stage and status' },
  { label: 'Persons', value: 'persons', description: 'List CRM contacts/persons' },
  { label: 'Organizations', value: 'organizations', description: 'List CRM companies/organizations' },
  { label: 'Products', value: 'products', description: 'List CRM products' },
  { label: 'Count', value: 'count', description: 'Count records for a chosen entity' },
];

export const COUNT_ENTITY_OPTIONS: Array<{ label: string; value: PipedriveCountEntity; description: string }> = [
  { label: 'Deals', value: 'deals', description: 'Count deals matching the filters' },
  { label: 'Persons', value: 'persons', description: 'Count persons' },
  { label: 'Organizations', value: 'organizations', description: 'Count organizations' },
  { label: 'Products', value: 'products', description: 'Count products' },
];

export const DEAL_STATUS_OPTIONS: Array<{ label: string; value: string; description: string }> = [
  { label: 'All', value: 'all', description: 'All deals regardless of status' },
  { label: 'Open', value: 'open', description: 'Only open deals' },
  { label: 'Won', value: 'won', description: 'Only won deals' },
  { label: 'Lost', value: 'lost', description: 'Only lost deals' },
  { label: 'Deleted', value: 'deleted', description: 'Deals deleted within the last 30 days' },
];

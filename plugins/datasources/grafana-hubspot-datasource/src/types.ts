import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type HubSpotQueryType =
  | 'contacts' | 'companies' | 'deals' | 'tickets'
  | 'products' | 'line_items'
  | 'meetings' | 'calls' | 'tasks' | 'notes' | 'emails'
  | 'pipelines' | 'owners' | 'properties' | 'raw';

export type HubSpotDateMode = 'any' | 'dashboard' | 'custom';

export interface Filter {
  propertyName: string;
  operator: string;
  value: string;
}

export interface FilterGroup {
  filters: Filter[];
}

export interface HubSpotQuery extends DataQuery {
  queryType: HubSpotQueryType;
  filterGroups?: FilterGroup[];
  sortBy?: string;
  sortDir?: 'ASCENDING' | 'DESCENDING';
  properties?: string[];
  pipelineId?: string;
  stageId?: string;
  createdMode?: HubSpotDateMode;
  createdAfter?: string;
  createdBefore?: string;
  updatedMode?: HubSpotDateMode;
  updatedAfter?: string;
  updatedBefore?: string;
  limit?: number;
  objectType?: string;
  rawPath?: string;
  rawMethod?: 'GET' | 'POST';
  rawBody?: string;
  rawRoot?: string;
}

export const DEFAULT_QUERY: Partial<HubSpotQuery> = {
  queryType: 'contacts',
  sortBy: 'createdate',
  sortDir: 'DESCENDING',
  limit: 0,
  createdMode: 'any',
  updatedMode: 'any',
};

export type HubSpotAuthMethod = 'privateApp' | 'oauth';

export interface HubSpotDataSourceOptions extends DataSourceJsonData {
  baseURL?: string;
  authMethod?: HubSpotAuthMethod;
}

export interface HubSpotSecureJsonData {
  privateAppToken?: string;
  oauthToken?: string;
}

// Resource types returned by the backend
export interface PropertyInfo {
  name: string;
  label: string;
  type: string;
}

export interface PipelineStageInfo {
  id: string;
  label: string;
}

export interface PipelineInfo {
  id: string;
  label: string;
  stages: PipelineStageInfo[];
}

export interface OwnerInfo {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
}

export const QUERY_TYPE_OPTIONS: Array<{ label: string; value: HubSpotQueryType; description: string }> = [
  { label: 'Contacts', value: 'contacts', description: 'List CRM contacts' },
  { label: 'Companies', value: 'companies', description: 'List CRM companies' },
  { label: 'Deals', value: 'deals', description: 'List CRM deals with pipeline and stage' },
  { label: 'Tickets', value: 'tickets', description: 'List CRM tickets' },
  { label: 'Products', value: 'products', description: 'List CRM products' },
  { label: 'Line Items', value: 'line_items', description: 'List line items' },
  { label: 'Meetings', value: 'meetings', description: 'List meeting engagements' },
  { label: 'Calls', value: 'calls', description: 'List call engagements' },
  { label: 'Tasks', value: 'tasks', description: 'List task engagements' },
  { label: 'Notes', value: 'notes', description: 'List note engagements' },
  { label: 'Emails', value: 'emails', description: 'List email engagements' },
  { label: 'Pipelines', value: 'pipelines', description: 'List pipelines and stages' },
  { label: 'Owners', value: 'owners', description: 'List account owners' },
  { label: 'Properties', value: 'properties', description: 'List property definitions' },
  { label: 'Raw REST', value: 'raw', description: 'Custom REST API call' },
];

export const DATE_MODE_OPTIONS: Array<{ label: string; value: HubSpotDateMode; description: string }> = [
  { label: 'Any time', value: 'any', description: 'Do not filter by this date' },
  { label: 'Dashboard range', value: 'dashboard', description: "Use the dashboard's time range" },
  { label: 'Custom', value: 'custom', description: 'Enter explicit after/before bounds' },
];

export const SEARCH_OPERATORS: Array<{ label: string; value: string }> = [
  { label: 'Equals (EQ)', value: 'EQ' },
  { label: 'Not equals (NEQ)', value: 'NEQ' },
  { label: 'Greater than (GT)', value: 'GT' },
  { label: 'Greater than or equal (GTE)', value: 'GTE' },
  { label: 'Less than (LT)', value: 'LT' },
  { label: 'Less than or equal (LTE)', value: 'LTE' },
  { label: 'Between (BETWEEN)', value: 'BETWEEN' },
  { label: 'In (IN)', value: 'IN' },
  { label: 'Not in (NOT_IN)', value: 'NOT_IN' },
  { label: 'Has property', value: 'HAS_PROPERTY' },
  { label: 'Not has property', value: 'NOT_HAS_PROPERTY' },
  { label: 'Contains token', value: 'CONTAINS_TOKEN' },
  { label: 'Not contains token', value: 'NOT_CONTAINS_TOKEN' },
  { label: 'Starts with', value: 'STARTS_WITH' },
  { label: 'Starts with token', value: 'STARTS_WITH_TOKEN' },
];

export const PIPELINE_OBJECT_TYPES = ['deals', 'tickets'];
export const DATE_FILTER_OBJECT_TYPES = ['contacts', 'companies', 'deals', 'tickets', 'products', 'line_items'];

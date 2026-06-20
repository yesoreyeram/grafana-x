import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { HubSpotQuery, HubSpotDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, HubSpotQuery, HubSpotDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

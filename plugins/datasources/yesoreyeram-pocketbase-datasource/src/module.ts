import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { PocketBaseQuery, PocketBaseDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, PocketBaseQuery, PocketBaseDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { AppwriteQuery, AppwriteDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, AppwriteQuery, AppwriteDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

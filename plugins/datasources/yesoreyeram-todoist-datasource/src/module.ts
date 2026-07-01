import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { TodoistQuery, TodoistDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, TodoistQuery, TodoistDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

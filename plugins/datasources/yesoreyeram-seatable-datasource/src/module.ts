import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { SeaTableQuery, SeaTableDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, SeaTableQuery, SeaTableDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

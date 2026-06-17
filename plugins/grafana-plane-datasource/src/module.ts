import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { PlaneQuery, PlaneDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, PlaneQuery, PlaneDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

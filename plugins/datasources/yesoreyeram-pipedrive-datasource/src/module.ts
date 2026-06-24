import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { PipedriveQuery, PipedriveDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, PipedriveQuery, PipedriveDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { IntercomQuery, IntercomDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, IntercomQuery, IntercomDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

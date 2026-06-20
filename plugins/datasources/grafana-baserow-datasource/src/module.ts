import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { BaserowQuery, BaserowDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, BaserowQuery, BaserowDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

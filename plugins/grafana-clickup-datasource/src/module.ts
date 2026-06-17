import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { ClickUpQuery, ClickUpDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, ClickUpQuery, ClickUpDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

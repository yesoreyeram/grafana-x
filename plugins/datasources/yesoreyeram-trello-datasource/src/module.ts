import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { TrelloQuery, TrelloDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, TrelloQuery, TrelloDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

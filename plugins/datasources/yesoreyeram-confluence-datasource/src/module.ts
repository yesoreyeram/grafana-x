import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { ConfluenceQuery, ConfluenceDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, ConfluenceQuery, ConfluenceDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { MondayQuery, MondayDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, MondayQuery, MondayDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

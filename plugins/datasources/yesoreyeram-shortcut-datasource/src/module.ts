import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { ShortcutQuery, ShortcutDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, ShortcutQuery, ShortcutDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

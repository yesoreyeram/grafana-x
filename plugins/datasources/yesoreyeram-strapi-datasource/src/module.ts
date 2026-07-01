import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { StrapiQuery, StrapiDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, StrapiQuery, StrapiDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

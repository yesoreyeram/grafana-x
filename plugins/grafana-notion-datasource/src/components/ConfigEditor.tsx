import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { NotionDataSourceOptions, NotionSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<NotionDataSourceOptions, NotionSecureJsonData> {}

const LABEL_WIDTH = 24;
const INPUT_WIDTH = 50;

const NOTION_API_URL = 'https://api.notion.com';
const DEFAULT_NOTION_VERSION = '2022-06-28';

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, baseURL: event.target.value },
    });
  };

  const onNotionVersionChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, notionVersion: event.target.value },
    });
  };

  const onDatabaseIdChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, databaseId: event.target.value },
    });
  };

  const onTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: { ...secureJsonData, apiToken: event.target.value },
    });
  };

  const onResetToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, apiToken: false },
      secureJsonData: { ...secureJsonData, apiToken: '' },
    });
  };

  return (
    <FieldSet label="Notion Connection">
      <InlineField
        label="API URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Root URL of the Notion API. Defaults to https://api.notion.com. Override to point at a proxy."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder={NOTION_API_URL}
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>

      <InlineField
        label="Notion-Version"
        labelWidth={LABEL_WIDTH}
        tooltip="Value of the Notion-Version header sent with every request."
      >
        <Input
          width={INPUT_WIDTH}
          name="notionVersion"
          placeholder={DEFAULT_NOTION_VERSION}
          value={jsonData.notionVersion ?? ''}
          onChange={onNotionVersionChange}
        />
      </InlineField>

      <InlineField
        label="Integration Token"
        labelWidth={LABEL_WIDTH}
        tooltip="Notion internal integration token, sent as the Authorization: Bearer header"
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiToken)}
          value={secureJsonData?.apiToken ?? ''}
          placeholder="secret_xxxxxxxxxxxx"
          onReset={onResetToken}
          onChange={onTokenChange}
        />
      </InlineField>

      <InlineField
        label="Default Database ID"
        labelWidth={LABEL_WIDTH}
        tooltip="Optional. A Notion database id used to prefill the query editor."
      >
        <Input
          width={INPUT_WIDTH}
          name="databaseId"
          placeholder="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
          value={jsonData.databaseId ?? ''}
          onChange={onDatabaseIdChange}
        />
      </InlineField>
    </FieldSet>
  );
}

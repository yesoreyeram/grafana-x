import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { AppwriteDataSourceOptions, AppwriteSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<AppwriteDataSourceOptions, AppwriteSecureJsonData> {}

const LABEL_WIDTH = 22;
const INPUT_WIDTH = 50;

const APPWRITE_DEFAULT_URL = 'https://cloud.appwrite.io/v1';

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onEndpointChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, endpoint: event.target.value },
    });
  };

  const onProjectIdChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, projectId: event.target.value },
    });
  };

  const onDatabaseIdChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, databaseId: event.target.value },
    });
  };

  const onKeyChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: { ...secureJsonData, apiKey: event.target.value },
    });
  };

  const onResetKey = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, apiKey: false },
      secureJsonData: { ...secureJsonData, apiKey: '' },
    });
  };

  return (
    <FieldSet label="Appwrite Connection">
      <InlineField
        label="Endpoint"
        labelWidth={LABEL_WIDTH}
        tooltip="Appwrite API endpoint, including the /v1 suffix. For Appwrite Cloud use https://cloud.appwrite.io/v1 (or a regional endpoint like https://nyc.cloud.appwrite.io/v1). For self-hosted, use https://<your-host>/v1."
        required
      >
        <Input
          width={INPUT_WIDTH}
          name="endpoint"
          placeholder={APPWRITE_DEFAULT_URL}
          value={jsonData.endpoint ?? ''}
          onChange={onEndpointChange}
        />
      </InlineField>

      <InlineField
        label="Project ID"
        labelWidth={LABEL_WIDTH}
        tooltip="Appwrite project id, sent in the X-Appwrite-Project header. Find it in the Appwrite console under your project settings."
        required
      >
        <Input
          width={INPUT_WIDTH}
          name="projectId"
          placeholder="Enter Appwrite project id"
          value={jsonData.projectId ?? ''}
          onChange={onProjectIdChange}
        />
      </InlineField>

      <InlineField
        label="API Key"
        labelWidth={LABEL_WIDTH}
        tooltip="Appwrite API key, sent in the X-Appwrite-Key header. Create one in the Appwrite console (Overview > Integrations > API keys) with the scopes databases.read, collections.read, attributes.read and documents.read."
        required
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiKey)}
          value={secureJsonData?.apiKey ?? ''}
          placeholder="Enter Appwrite API key"
          onReset={onResetKey}
          onChange={onKeyChange}
        />
      </InlineField>

      <InlineField
        label="Default Database ID"
        labelWidth={LABEL_WIDTH}
        tooltip="Optional. Appwrite database id. When set, the query editor lists this database's collections directly. Otherwise you can pick a database in the query editor."
      >
        <Input
          width={INPUT_WIDTH}
          name="databaseId"
          placeholder="database id (optional)"
          value={jsonData.databaseId ?? ''}
          onChange={onDatabaseIdChange}
        />
      </InlineField>
    </FieldSet>
  );
}

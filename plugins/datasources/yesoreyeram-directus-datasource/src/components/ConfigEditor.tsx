import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { DirectusDataSourceOptions, DirectusSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<DirectusDataSourceOptions, DirectusSecureJsonData> {}

const LABEL_WIDTH = 22;
const INPUT_WIDTH = 50;

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, baseURL: event.target.value },
    });
  };

  const onDefaultCollectionChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, defaultCollectionId: event.target.value },
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
    <FieldSet label="Directus Connection">
      <InlineField
        label="API URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Directus API base URL (e.g. https://your-directus.example.com). This is required since Directus is self-hosted."
        required
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder="https://your-directus.example.com"
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>

      <InlineField
        label="API Token"
        labelWidth={LABEL_WIDTH}
        tooltip="Directus static API token, sent as 'Authorization: Bearer <token>'. Generate one in the Directus admin panel under Settings > API Tokens."
        required
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiToken)}
          value={secureJsonData?.apiToken ?? ''}
          placeholder="Enter Directus API token"
          onReset={onResetToken}
          onChange={onTokenChange}
        />
      </InlineField>

      <InlineField
        label="Default Collection"
        labelWidth={LABEL_WIDTH}
        tooltip="Optional. Directus collection name. When set, the query editor selects this collection by default."
      >
        <Input
          width={INPUT_WIDTH}
          name="defaultCollectionId"
          placeholder="articles (optional)"
          value={jsonData.defaultCollectionId ?? ''}
          onChange={onDefaultCollectionChange}
        />
      </InlineField>
    </FieldSet>
  );
}

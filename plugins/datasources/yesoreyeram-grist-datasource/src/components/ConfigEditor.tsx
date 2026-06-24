import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { GristDataSourceOptions, GristSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<GristDataSourceOptions, GristSecureJsonData> {}

const LABEL_WIDTH = 22;
const INPUT_WIDTH = 50;

const GRIST_DEFAULT_URL = 'http://localhost:8484';

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, baseURL: event.target.value },
    });
  };

  const onDocIdChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, docId: event.target.value },
    });
  };

  const onApiKeyChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: { ...secureJsonData, apiKey: event.target.value },
    });
  };

  const onResetApiKey = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, apiKey: false },
      secureJsonData: { ...secureJsonData, apiKey: '' },
    });
  };

  return (
    <FieldSet label="Grist Connection">
      <InlineField
        label="API Key"
        labelWidth={LABEL_WIDTH}
        tooltip="Grist API key, sent as 'Authorization: Bearer <key>'. Create one in your Grist account settings."
        required
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiKey)}
          value={secureJsonData?.apiKey ?? ''}
          placeholder="Enter Grist API key"
          onReset={onResetApiKey}
          onChange={onApiKeyChange}
        />
      </InlineField>

      <InlineField
        label="Default Doc ID"
        labelWidth={LABEL_WIDTH}
        tooltip="Optional. Grist document id. When set, the query editor lists this doc's tables directly."
      >
        <Input
          width={INPUT_WIDTH}
          name="docId"
          placeholder="Enter default document id (optional)"
          value={jsonData.docId ?? ''}
          onChange={onDocIdChange}
        />
      </InlineField>

      <InlineField
        label="Grist URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Grist server URL. Cloud team sites: https://{team}.getgrist.com. Self-hosted: the instance URL (e.g. http://localhost:8484). A trailing /api is optional. Defaults to http://localhost:8484."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder={GRIST_DEFAULT_URL}
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>
    </FieldSet>
  );
}

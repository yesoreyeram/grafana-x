import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { AsanaDataSourceOptions, AsanaSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<AsanaDataSourceOptions, AsanaSecureJsonData> {}

const LABEL_WIDTH = 24;
const INPUT_WIDTH = 50;

const ASANA_API_URL = 'https://app.asana.com/api/1.0';

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, baseURL: event.target.value },
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
    <FieldSet label="Asana Connection">
      <InlineField
        label="API URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Asana API root. Defaults to https://app.asana.com/api/1.0. Override only to point at a proxy."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder={ASANA_API_URL}
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>

      <InlineField
        label="Personal Access Token"
        labelWidth={LABEL_WIDTH}
        tooltip="An Asana personal access token (or OAuth access token), created at My Settings > Apps > Developer apps > Manage developer apps. Sent as the Authorization: Bearer header."
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiKey)}
          value={secureJsonData?.apiKey ?? ''}
          placeholder="1/1234567890:abcdef…"
          onReset={onResetApiKey}
          onChange={onApiKeyChange}
        />
      </InlineField>
    </FieldSet>
  );
}

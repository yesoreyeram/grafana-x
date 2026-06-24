import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { TeableDataSourceOptions, TeableSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<TeableDataSourceOptions, TeableSecureJsonData> {}

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

  const onDefaultBaseIdChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, defaultBaseId: event.target.value },
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
    <FieldSet label="Teable Connection">
      <InlineField
        label="Server URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Teable API base URL. Defaults to https://app.teable.io (cloud); set your own domain for a self-hosted instance."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder="https://app.teable.io"
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>

      <InlineField
        label="API Token"
        labelWidth={LABEL_WIDTH}
        tooltip="Teable API token, sent as 'Authorization: Bearer <token>'. Generate one from your Teable account settings."
        required
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiToken)}
          value={secureJsonData?.apiToken ?? ''}
          placeholder="Enter Teable API token"
          onReset={onResetToken}
          onChange={onTokenChange}
        />
      </InlineField>

      <InlineField
        label="Default Base ID"
        labelWidth={LABEL_WIDTH}
        tooltip="Optional. Teable base ID. When set, the query editor selects this base by default."
      >
        <Input
          width={INPUT_WIDTH}
          name="defaultBaseId"
          placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx (optional)"
          value={jsonData.defaultBaseId ?? ''}
          onChange={onDefaultBaseIdChange}
        />
      </InlineField>
    </FieldSet>
  );
}

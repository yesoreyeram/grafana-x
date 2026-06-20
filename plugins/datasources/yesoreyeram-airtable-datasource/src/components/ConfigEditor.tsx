import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { AirtableDataSourceOptions, AirtableSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<AirtableDataSourceOptions, AirtableSecureJsonData> {}

const LABEL_WIDTH = 22;
const INPUT_WIDTH = 50;

const AIRTABLE_DEFAULT_URL = 'https://api.airtable.com';

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, baseURL: event.target.value },
    });
  };

  const onBaseIdChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, baseId: event.target.value },
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
    <FieldSet label="Airtable Connection">
      <InlineField
        label="Personal Access Token"
        labelWidth={LABEL_WIDTH}
        tooltip="Airtable personal access token (PAT), sent as 'Authorization: Bearer <token>'. Create one at airtable.com/create/tokens with the data.records:read and schema.bases:read scopes."
        required
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiToken)}
          value={secureJsonData?.apiToken ?? ''}
          placeholder="Enter Airtable personal access token"
          onReset={onResetToken}
          onChange={onTokenChange}
        />
      </InlineField>

      <InlineField
        label="Default Base ID"
        labelWidth={LABEL_WIDTH}
        tooltip="Optional. Airtable base id (starts with 'app...'). When set, the query editor lists this base's tables directly. Otherwise you can pick a base in the query editor."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseId"
          placeholder="appXXXXXXXXXXXXXX (optional)"
          value={jsonData.baseId ?? ''}
          onChange={onBaseIdChange}
        />
      </InlineField>

      <InlineField
        label="API URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Airtable API base URL. Leave as the default unless routing through a proxy."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder={AIRTABLE_DEFAULT_URL}
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>
    </FieldSet>
  );
}

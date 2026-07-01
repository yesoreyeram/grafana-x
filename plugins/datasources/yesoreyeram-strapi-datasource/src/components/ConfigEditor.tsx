import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet, RadioButtonGroup } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';

import { StrapiApiVersion, StrapiDataSourceOptions, StrapiSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<StrapiDataSourceOptions, StrapiSecureJsonData> {}

const LABEL_WIDTH = 22;
const INPUT_WIDTH = 50;

const API_VERSION_OPTIONS: Array<SelectableValue<StrapiApiVersion>> = [
  { label: 'v5 (default)', value: 'v5', description: 'Flat response; documents addressed by documentId' },
  { label: 'v4', value: 'v4', description: 'Fields nested under an attributes object' },
];

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, baseURL: event.target.value },
    });
  };

  const onDefaultContentTypeChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, defaultContentTypeId: event.target.value },
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

  const onApiVersionChange = (apiVersion: StrapiApiVersion) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, apiVersion },
    });
  };

  return (
    <FieldSet label="Strapi Connection">
      <InlineField
        label="API URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Strapi API base URL (e.g. https://your-strapi.example.com). Do not include the /api suffix."
        required
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder="https://your-strapi.example.com"
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>

      <InlineField
        label="API Version"
        labelWidth={LABEL_WIDTH}
        tooltip="Strapi major version. v5 returns flat records with a documentId; v4 nests fields under attributes. The backend auto-detects the shape per record, so this is a hint (default v5)."
      >
        <RadioButtonGroup<StrapiApiVersion>
          options={API_VERSION_OPTIONS}
          value={jsonData.apiVersion ?? 'v5'}
          onChange={onApiVersionChange}
        />
      </InlineField>

      <InlineField
        label="API Token"
        labelWidth={LABEL_WIDTH}
        tooltip="Strapi API token, sent as 'Authorization: Bearer <token>'. Generate one in the Strapi admin panel under Settings > API Tokens."
        required
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiToken)}
          value={secureJsonData?.apiToken ?? ''}
          placeholder="Enter Strapi API token"
          onReset={onResetToken}
          onChange={onTokenChange}
        />
      </InlineField>

      <InlineField
        label="Default Content Type"
        labelWidth={LABEL_WIDTH}
        tooltip="Optional. Strapi content type plural API id (e.g. 'articles'). When set, the query editor selects it by default and Save & test validates the API token against it (otherwise the health check only verifies the base URL is reachable)."
      >
        <Input
          width={INPUT_WIDTH}
          name="defaultContentTypeId"
          placeholder="articles (optional)"
          value={jsonData.defaultContentTypeId ?? ''}
          onChange={onDefaultContentTypeChange}
        />
      </InlineField>
    </FieldSet>
  );
}

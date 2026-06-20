import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet, RadioButtonGroup, Select } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';

import {
  NocoDBApiVersion,
  NocoDBDataSourceOptions,
  NocoDBPlatform,
  NocoDBSecureJsonData,
} from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<NocoDBDataSourceOptions, NocoDBSecureJsonData> {}

const LABEL_WIDTH = 22;
const INPUT_WIDTH = 50;

const NOCODB_CLOUD_URL = 'https://app.nocodb.com';

const PLATFORM_OPTIONS: Array<SelectableValue<NocoDBPlatform>> = [
  { label: 'NocoDB Cloud', value: 'cloud' },
  { label: 'Self-hosted', value: 'selfhosted' },
];

const API_VERSION_OPTIONS: Array<SelectableValue<NocoDBApiVersion>> = [
  { label: 'v2', value: 'v2', description: 'NocoDB Data API v2 (recommended, widely available)' },
  { label: 'v3', value: 'v3', description: 'NocoDB Data API v3 (requires a base id)' },
];

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const platform: NocoDBPlatform = jsonData.platform ?? 'selfhosted';
  const apiVersion: NocoDBApiVersion = jsonData.apiVersion ?? 'v2';

  const onPlatformChange = (value: NocoDBPlatform) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        platform: value,
        // Cloud forces the canonical NocoDB cloud URL.
        baseURL: value === 'cloud' ? NOCODB_CLOUD_URL : jsonData.baseURL,
      },
    });
  };

  const onApiVersionChange = (value: SelectableValue<NocoDBApiVersion> | null) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, apiVersion: value?.value ?? 'v2' },
    });
  };

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
    <FieldSet label="NocoDB Connection">
      <InlineField
        label="Platform"
        labelWidth={LABEL_WIDTH}
        tooltip="NocoDB Cloud uses app.nocodb.com. Choose Self-hosted to point at your own instance."
      >
        <RadioButtonGroup<NocoDBPlatform>
          options={PLATFORM_OPTIONS}
          value={platform}
          onChange={onPlatformChange}
        />
      </InlineField>

      <InlineField
        label="Base URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Root URL of your NocoDB instance. Fixed to app.nocodb.com for NocoDB Cloud."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder="https://nocodb.example.com"
          value={platform === 'cloud' ? NOCODB_CLOUD_URL : jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
          disabled={platform === 'cloud'}
        />
      </InlineField>

      <InlineField
        label="API Version"
        labelWidth={LABEL_WIDTH}
        tooltip="NocoDB data API version used for record queries."
      >
        <Select<NocoDBApiVersion>
          width={INPUT_WIDTH}
          options={API_VERSION_OPTIONS}
          value={API_VERSION_OPTIONS.find((o) => o.value === apiVersion)}
          onChange={onApiVersionChange}
        />
      </InlineField>

      <InlineField label="API Token" labelWidth={LABEL_WIDTH} tooltip="NocoDB API token, sent as the xc-token header">
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiToken)}
          value={secureJsonData?.apiToken ?? ''}
          placeholder="Enter NocoDB API token"
          onReset={onResetToken}
          onChange={onTokenChange}
        />
      </InlineField>

      <InlineField
        label="Default Base ID"
        labelWidth={LABEL_WIDTH}
        tooltip="Optional. NocoDB base id (prefixed with p) used to list tables and required for the v3 API."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseId"
          placeholder="p_xxxxxxxxxxxx"
          value={jsonData.baseId ?? ''}
          onChange={onBaseIdChange}
        />
      </InlineField>
    </FieldSet>
  );
}

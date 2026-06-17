import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet, RadioButtonGroup } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';

import { MondayAuthMethod, MondayDataSourceOptions, MondaySecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<MondayDataSourceOptions, MondaySecureJsonData> {}

const LABEL_WIDTH = 24;
const INPUT_WIDTH = 50;

const MONDAY_API_URL = 'https://api.monday.com/v2';

const AUTH_OPTIONS: Array<SelectableValue<MondayAuthMethod>> = [
  {
    label: 'Personal API token',
    value: 'apiKey',
    description: 'monday.com personal API token, sent as the Authorization header',
  },
  { label: 'OAuth token', value: 'oauth', description: 'monday.com OAuth2 access token, sent as Authorization: Bearer' },
];

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;
  const authMethod: MondayAuthMethod = jsonData.authMethod ?? 'apiKey';

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, baseURL: event.target.value },
    });
  };

  const onApiVersionChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, apiVersion: event.target.value },
    });
  };

  const onAuthMethodChange = (value: MondayAuthMethod) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, authMethod: value },
    });
  };

  const onApiTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: { ...secureJsonData, apiToken: event.target.value },
    });
  };

  const onResetApiToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, apiToken: false },
      secureJsonData: { ...secureJsonData, apiToken: '' },
    });
  };

  const onOAuthTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: { ...secureJsonData, oauthToken: event.target.value },
    });
  };

  const onResetOAuthToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, oauthToken: false },
      secureJsonData: { ...secureJsonData, oauthToken: '' },
    });
  };

  return (
    <FieldSet label="monday.com Connection">
      <InlineField
        label="GraphQL URL"
        labelWidth={LABEL_WIDTH}
        tooltip="monday.com GraphQL endpoint. Defaults to https://api.monday.com/v2. Override only to point at a proxy."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder={MONDAY_API_URL}
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>

      <InlineField
        label="API version"
        labelWidth={LABEL_WIDTH}
        tooltip="Optional monday.com API version (e.g. 2026-01), sent as the API-Version header. Leave empty to use a recent bundled default (2026-01). Grouping/aggregation requires a version that exposes the aggregate API (2026-01 or later); older versions return a clear error."
      >
        <Input
          width={INPUT_WIDTH}
          name="apiVersion"
          placeholder="2026-01"
          value={jsonData.apiVersion ?? ''}
          onChange={onApiVersionChange}
        />
      </InlineField>

      <InlineField
        label="Authentication"
        labelWidth={LABEL_WIDTH}
        tooltip="How to authenticate to monday.com: a personal API token or an OAuth2 access token."
      >
        <RadioButtonGroup<MondayAuthMethod> options={AUTH_OPTIONS} value={authMethod} onChange={onAuthMethodChange} />
      </InlineField>

      {authMethod === 'apiKey' && (
        <InlineField
          label="API Token"
          labelWidth={LABEL_WIDTH}
          tooltip="monday.com personal API token, found in your avatar menu > Developers > My Access Tokens (or Admin > API). Sent as the Authorization header."
        >
          <SecretInput
            width={INPUT_WIDTH}
            isConfigured={Boolean(secureJsonFields?.apiToken)}
            value={secureJsonData?.apiToken ?? ''}
            placeholder="eyJhbGciOi..."
            onReset={onResetApiToken}
            onChange={onApiTokenChange}
          />
        </InlineField>
      )}

      {authMethod === 'oauth' && (
        <InlineField
          label="OAuth Token"
          labelWidth={LABEL_WIDTH}
          tooltip="monday.com OAuth2 access token, sent as the Authorization: Bearer header."
        >
          <SecretInput
            width={INPUT_WIDTH}
            isConfigured={Boolean(secureJsonFields?.oauthToken)}
            value={secureJsonData?.oauthToken ?? ''}
            placeholder="oauth access token"
            onReset={onResetOAuthToken}
            onChange={onOAuthTokenChange}
          />
        </InlineField>
      )}
    </FieldSet>
  );
}

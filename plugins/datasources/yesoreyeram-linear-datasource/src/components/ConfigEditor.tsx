import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet, RadioButtonGroup } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';

import { LinearAuthMethod, LinearDataSourceOptions, LinearSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<LinearDataSourceOptions, LinearSecureJsonData> {}

const LABEL_WIDTH = 24;
const INPUT_WIDTH = 50;

const LINEAR_API_URL = 'https://api.linear.app/graphql';

const AUTH_OPTIONS: Array<SelectableValue<LinearAuthMethod>> = [
  { label: 'Personal API key', value: 'apiKey', description: 'Linear personal API key, sent as the Authorization header' },
  { label: 'OAuth token', value: 'oauth', description: 'Linear OAuth2 access token, sent as Authorization: Bearer' },
];

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;
  const authMethod: LinearAuthMethod = jsonData.authMethod ?? 'apiKey';

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, baseURL: event.target.value },
    });
  };

  const onAuthMethodChange = (value: LinearAuthMethod) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, authMethod: value },
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
    <FieldSet label="Linear Connection">
      <InlineField
        label="GraphQL URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Linear GraphQL endpoint. Defaults to https://api.linear.app/graphql. Override only to point at a proxy."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder={LINEAR_API_URL}
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>

      <InlineField
        label="Authentication"
        labelWidth={LABEL_WIDTH}
        tooltip="How to authenticate to Linear: a personal API key or an OAuth2 access token."
      >
        <RadioButtonGroup<LinearAuthMethod>
          options={AUTH_OPTIONS}
          value={authMethod}
          onChange={onAuthMethodChange}
        />
      </InlineField>

      {authMethod === 'apiKey' && (
        <InlineField
          label="API Key"
          labelWidth={LABEL_WIDTH}
          tooltip="Linear personal API key, created at Settings > Security & access > Personal API keys. Sent as the Authorization header."
        >
          <SecretInput
            width={INPUT_WIDTH}
            isConfigured={Boolean(secureJsonFields?.apiKey)}
            value={secureJsonData?.apiKey ?? ''}
            placeholder="lin_api_xxxxxxxxxxxx"
            onReset={onResetApiKey}
            onChange={onApiKeyChange}
          />
        </InlineField>
      )}

      {authMethod === 'oauth' && (
        <InlineField
          label="OAuth Token"
          labelWidth={LABEL_WIDTH}
          tooltip="Linear OAuth2 access token, sent as the Authorization: Bearer header."
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

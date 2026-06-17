import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet, RadioButtonGroup } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';

import { ClickUpAuthMethod, ClickUpDataSourceOptions, ClickUpSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<ClickUpDataSourceOptions, ClickUpSecureJsonData> {}

const LABEL_WIDTH = 24;
const INPUT_WIDTH = 50;

const CLICKUP_API_URL = 'https://api.clickup.com/api';

const AUTH_OPTIONS: Array<SelectableValue<ClickUpAuthMethod>> = [
  {
    label: 'Personal token',
    value: 'apiKey',
    description: 'ClickUp personal API token (pk_...), sent as the Authorization header',
  },
  {
    label: 'OAuth token',
    value: 'oauth',
    description: 'ClickUp OAuth2 access token, sent as Authorization: Bearer',
  },
];

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;
  const authMethod: ClickUpAuthMethod = jsonData.authMethod ?? 'apiKey';

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, baseURL: event.target.value },
    });
  };

  const onAuthMethodChange = (value: ClickUpAuthMethod) => {
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
    <FieldSet label="ClickUp Connection">
      <InlineField
        label="API URL"
        labelWidth={LABEL_WIDTH}
        tooltip="ClickUp API root. Defaults to https://api.clickup.com/api. Override only to point at a proxy."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder={CLICKUP_API_URL}
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>

      <InlineField
        label="Authentication"
        labelWidth={LABEL_WIDTH}
        tooltip="How to authenticate to ClickUp: a personal API token or an OAuth2 access token."
      >
        <RadioButtonGroup<ClickUpAuthMethod>
          options={AUTH_OPTIONS}
          value={authMethod}
          onChange={onAuthMethodChange}
        />
      </InlineField>

      {authMethod === 'apiKey' && (
        <InlineField
          label="Personal token"
          labelWidth={LABEL_WIDTH}
          tooltip="ClickUp personal API token, created at Settings > Apps > API Token. Sent as the Authorization header."
        >
          <SecretInput
            width={INPUT_WIDTH}
            isConfigured={Boolean(secureJsonFields?.apiKey)}
            value={secureJsonData?.apiKey ?? ''}
            placeholder="pk_xxxxxxxxxxxx"
            onReset={onResetApiKey}
            onChange={onApiKeyChange}
          />
        </InlineField>
      )}

      {authMethod === 'oauth' && (
        <InlineField
          label="OAuth Token"
          labelWidth={LABEL_WIDTH}
          tooltip="ClickUp OAuth2 access token, sent as the Authorization: Bearer header."
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

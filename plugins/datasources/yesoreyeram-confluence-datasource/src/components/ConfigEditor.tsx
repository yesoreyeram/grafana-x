import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet, RadioButtonGroup } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';

import { ConfluenceAuthMode, ConfluenceDataSourceOptions, ConfluenceSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<ConfluenceDataSourceOptions, ConfluenceSecureJsonData> {}

const LABEL_WIDTH = 24;
const INPUT_WIDTH = 50;
const BASE_URL_PLACEHOLDER = 'https://your-site.atlassian.net/wiki';

const AUTH_OPTIONS: Array<SelectableValue<ConfluenceAuthMode>> = [
  { label: 'Basic (email + API token)', value: 'basic', description: 'Atlassian Cloud: account email + API token, sent as Basic auth' },
  { label: 'Bearer (OAuth2 / PAT)', value: 'bearer', description: 'OAuth2 access token or Data Center Personal Access Token, sent as Bearer' },
];

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;
  const authMode: ConfluenceAuthMode = jsonData.authMode ?? 'basic';

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, baseURL: event.target.value } });
  };
  const onAuthModeChange = (value: ConfluenceAuthMode) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, authMode: value } });
  };
  const onEmailChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, email: event.target.value } });
  };
  const onApiTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, secureJsonData: { ...secureJsonData, apiToken: event.target.value } });
  };
  const onResetApiToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, apiToken: false },
      secureJsonData: { ...secureJsonData, apiToken: '' },
    });
  };
  const onBearerTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, secureJsonData: { ...secureJsonData, bearerToken: event.target.value } });
  };
  const onResetBearerToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, bearerToken: false },
      secureJsonData: { ...secureJsonData, bearerToken: '' },
    });
  };

  return (
    <FieldSet label="Confluence Connection">
      <InlineField
        label="Base URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Root URL of the Confluence wiki. For Cloud use https://your-site.atlassian.net/wiki (include /wiki). The v2 API path and the CQL search path are appended to this base."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder={BASE_URL_PLACEHOLDER}
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>

      <InlineField label="Authentication" labelWidth={LABEL_WIDTH} tooltip="How requests are authenticated against Confluence.">
        <RadioButtonGroup<ConfluenceAuthMode> options={AUTH_OPTIONS} value={authMode} onChange={onAuthModeChange} />
      </InlineField>

      {authMode === 'basic' && (
        <>
          <InlineField label="Email" labelWidth={LABEL_WIDTH} tooltip="Atlassian account email used for Basic auth.">
            <Input
              width={INPUT_WIDTH}
              name="email"
              placeholder="you@example.com"
              value={jsonData.email ?? ''}
              onChange={onEmailChange}
            />
          </InlineField>
          <InlineField
            label="API Token"
            labelWidth={LABEL_WIDTH}
            tooltip="Atlassian API token. Create one at id.atlassian.com/manage-profile/security/api-tokens."
          >
            <SecretInput
              width={INPUT_WIDTH}
              isConfigured={Boolean(secureJsonFields?.apiToken)}
              value={secureJsonData?.apiToken ?? ''}
              placeholder="API token"
              onReset={onResetApiToken}
              onChange={onApiTokenChange}
            />
          </InlineField>
        </>
      )}

      {authMode === 'bearer' && (
        <InlineField
          label="Token"
          labelWidth={LABEL_WIDTH}
          tooltip="OAuth2 access token (Cloud) or Personal Access Token (Data Center), sent as Authorization: Bearer."
        >
          <SecretInput
            width={INPUT_WIDTH}
            isConfigured={Boolean(secureJsonFields?.bearerToken)}
            value={secureJsonData?.bearerToken ?? ''}
            placeholder="access token"
            onReset={onResetBearerToken}
            onChange={onBearerTokenChange}
          />
        </InlineField>
      )}
    </FieldSet>
  );
}

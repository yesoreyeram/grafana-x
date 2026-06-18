import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet, RadioButtonGroup } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';
import { HubSpotAuthMethod, HubSpotDataSourceOptions, HubSpotSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<HubSpotDataSourceOptions, HubSpotSecureJsonData> {}

const LABEL_WIDTH = 24;
const INPUT_WIDTH = 50;
const HS_API_URL = 'https://api.hubapi.com';

const AUTH_OPTIONS: Array<SelectableValue<HubSpotAuthMethod>> = [
  { label: 'Private app token', value: 'privateApp', description: 'HubSpot private app access token, sent as Bearer token' },
  { label: 'OAuth token', value: 'oauth', description: 'HubSpot OAuth2 access token, sent as Bearer token' },
];

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;
  const authMethod: HubSpotAuthMethod = jsonData.authMethod ?? 'privateApp';

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, baseURL: event.target.value } });
  };
  const onAuthMethodChange = (value: HubSpotAuthMethod) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, authMethod: value } });
  };
  const onPrivateAppTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, secureJsonData: { ...secureJsonData, privateAppToken: event.target.value } });
  };
  const onResetPrivateAppToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, privateAppToken: false },
      secureJsonData: { ...secureJsonData, privateAppToken: '' },
    });
  };
  const onOAuthTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, secureJsonData: { ...secureJsonData, oauthToken: event.target.value } });
  };
  const onResetOAuthToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, oauthToken: false },
      secureJsonData: { ...secureJsonData, oauthToken: '' },
    });
  };

  return (
    <FieldSet label="HubSpot Connection">
      <InlineField
        label="API URL"
        labelWidth={LABEL_WIDTH}
        tooltip="HubSpot API root URL. Use https://api.hubapi.com for US or https://api.hubapi.eu for EU data residency."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder={HS_API_URL}
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>
      <InlineField label="Authentication" labelWidth={LABEL_WIDTH} tooltip="HubSpot authentication method.">
        <RadioButtonGroup<HubSpotAuthMethod> options={AUTH_OPTIONS} value={authMethod} onChange={onAuthMethodChange} />
      </InlineField>
      {authMethod === 'privateApp' && (
        <InlineField
          label="Private app token"
          labelWidth={LABEL_WIDTH}
          tooltip="HubSpot private app access token. Generate one at Settings > Integrations > Private Apps."
        >
          <SecretInput
            width={INPUT_WIDTH}
            isConfigured={Boolean(secureJsonFields?.privateAppToken)}
            value={secureJsonData?.privateAppToken ?? ''}
            placeholder="pat-xxxxxxxx-xxxx-xxxx"
            onReset={onResetPrivateAppToken}
            onChange={onPrivateAppTokenChange}
          />
        </InlineField>
      )}
      {authMethod === 'oauth' && (
        <InlineField label="OAuth Token" labelWidth={LABEL_WIDTH} tooltip="HubSpot OAuth2 access token.">
          <SecretInput
            width={INPUT_WIDTH}
            isConfigured={Boolean(secureJsonFields?.oauthToken)}
            value={secureJsonData?.oauthToken ?? ''}
            placeholder="access token"
            onReset={onResetOAuthToken}
            onChange={onOAuthTokenChange}
          />
        </InlineField>
      )}
    </FieldSet>
  );
}

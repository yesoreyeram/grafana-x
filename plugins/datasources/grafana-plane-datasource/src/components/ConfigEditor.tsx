import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet, RadioButtonGroup } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';

import { PlaneAuthMethod, PlaneDataSourceOptions, PlaneSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<PlaneDataSourceOptions, PlaneSecureJsonData> {}

const LABEL_WIDTH = 24;
const INPUT_WIDTH = 50;

const PLANE_API_URL = 'https://api.plane.so';

const AUTH_OPTIONS: Array<SelectableValue<PlaneAuthMethod>> = [
  {
    label: 'API key',
    value: 'apiKey',
    description: 'Plane personal API key, sent as the X-API-Key header',
  },
  {
    label: 'OAuth token',
    value: 'oauth',
    description: 'Plane OAuth2 access token, sent as Authorization: Bearer',
  },
];

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;
  const authMethod: PlaneAuthMethod = jsonData.authMethod ?? 'apiKey';

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, baseURL: event.target.value },
    });
  };

  const onWorkspaceSlugChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, workspaceSlug: event.target.value },
    });
  };

  const onAuthMethodChange = (value: PlaneAuthMethod) => {
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
    <FieldSet label="Plane Connection">
      <InlineField
        label="API URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Plane API root. Defaults to https://api.plane.so. Override it to point at a self-hosted instance or a proxy."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder={PLANE_API_URL}
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>

      <InlineField
        label="Workspace slug"
        labelWidth={LABEL_WIDTH}
        tooltip="Default workspace slug, e.g. the 'my-team' in https://app.plane.so/my-team/projects/. Used when a query does not set its own workspace."
      >
        <Input
          width={INPUT_WIDTH}
          name="workspaceSlug"
          placeholder="my-team"
          value={jsonData.workspaceSlug ?? ''}
          onChange={onWorkspaceSlugChange}
        />
      </InlineField>

      <InlineField
        label="Authentication"
        labelWidth={LABEL_WIDTH}
        tooltip="How to authenticate to Plane: a personal API key (X-API-Key) or an OAuth2 access token (Bearer)."
      >
        <RadioButtonGroup<PlaneAuthMethod> options={AUTH_OPTIONS} value={authMethod} onChange={onAuthMethodChange} />
      </InlineField>

      {authMethod === 'apiKey' && (
        <InlineField
          label="API key"
          labelWidth={LABEL_WIDTH}
          tooltip="Plane personal API key, created at Profile Settings > Personal Access Tokens. Sent as the X-API-Key header."
        >
          <SecretInput
            width={INPUT_WIDTH}
            isConfigured={Boolean(secureJsonFields?.apiKey)}
            value={secureJsonData?.apiKey ?? ''}
            placeholder="plane_api_xxxxxxxxxxxx"
            onReset={onResetApiKey}
            onChange={onApiKeyChange}
          />
        </InlineField>
      )}

      {authMethod === 'oauth' && (
        <InlineField
          label="OAuth Token"
          labelWidth={LABEL_WIDTH}
          tooltip="Plane OAuth2 access token, sent as the Authorization: Bearer header."
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

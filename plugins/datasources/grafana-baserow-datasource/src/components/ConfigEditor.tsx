import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet, RadioButtonGroup } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';

import { BaserowAuthMode, BaserowDataSourceOptions, BaserowPlatform, BaserowSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<BaserowDataSourceOptions, BaserowSecureJsonData> {}

const LABEL_WIDTH = 22;
const INPUT_WIDTH = 50;

const BASEROW_CLOUD_URL = 'https://api.baserow.io';

const PLATFORM_OPTIONS: Array<SelectableValue<BaserowPlatform>> = [
  { label: 'Baserow Cloud', value: 'cloud' },
  { label: 'Self-hosted', value: 'selfhosted' },
];

const AUTH_MODE_OPTIONS: Array<SelectableValue<BaserowAuthMode>> = [
  { label: 'Database token', value: 'token', description: 'A database token scoped to one database' },
  { label: 'Email & password', value: 'password', description: 'Sign in with credentials (JWT); lists all databases' },
];

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const platform: BaserowPlatform = jsonData.platform ?? 'selfhosted';
  const authMode: BaserowAuthMode = jsonData.authMode ?? 'token';
  const isPassword = authMode === 'password';

  const onPlatformChange = (value: BaserowPlatform) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        platform: value,
        // Cloud forces the canonical Baserow cloud URL.
        baseURL: value === 'cloud' ? BASEROW_CLOUD_URL : jsonData.baseURL,
      },
    });
  };

  const onAuthModeChange = (value: BaserowAuthMode) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, authMode: value },
    });
  };

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, baseURL: event.target.value },
    });
  };

  const onDatabaseIdChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, databaseId: event.target.value },
    });
  };

  const onEmailChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, email: event.target.value },
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

  const onPasswordChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: { ...secureJsonData, password: event.target.value },
    });
  };

  const onResetPassword = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, password: false },
      secureJsonData: { ...secureJsonData, password: '' },
    });
  };

  return (
    <FieldSet label="Baserow Connection">
      <InlineField
        label="Platform"
        labelWidth={LABEL_WIDTH}
        tooltip="Baserow Cloud uses api.baserow.io. Choose Self-hosted to point at your own instance."
      >
        <RadioButtonGroup<BaserowPlatform> options={PLATFORM_OPTIONS} value={platform} onChange={onPlatformChange} />
      </InlineField>

      <InlineField
        label="Base URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Root URL of your Baserow instance. Fixed to api.baserow.io for Baserow Cloud."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder="https://baserow.example.com"
          value={platform === 'cloud' ? BASEROW_CLOUD_URL : (jsonData.baseURL ?? '')}
          onChange={onBaseURLChange}
          disabled={platform === 'cloud'}
        />
      </InlineField>

      <InlineField
        label="Authentication"
        labelWidth={LABEL_WIDTH}
        tooltip="Database token is scoped to a single database. Email & password signs in for a JWT and can list every database you have access to."
      >
        <RadioButtonGroup<BaserowAuthMode> options={AUTH_MODE_OPTIONS} value={authMode} onChange={onAuthModeChange} />
      </InlineField>

      {!isPassword && (
        <>
          <InlineField
            label="API Token"
            labelWidth={LABEL_WIDTH}
            tooltip="Baserow database token, sent as the 'Authorization: Token <token>' header. Create one in Baserow settings."
          >
            <SecretInput
              width={INPUT_WIDTH}
              isConfigured={Boolean(secureJsonFields?.apiToken)}
              value={secureJsonData?.apiToken ?? ''}
              placeholder="Enter Baserow database token"
              onReset={onResetToken}
              onChange={onTokenChange}
            />
          </InlineField>

          <InlineField
            label="Database ID"
            labelWidth={LABEL_WIDTH}
            tooltip="Optional. Baserow database (application) id used to filter the table list. Leave empty to list every table the token can access."
          >
            <Input
              width={INPUT_WIDTH}
              name="databaseId"
              placeholder="optional"
              value={jsonData.databaseId ?? ''}
              onChange={onDatabaseIdChange}
            />
          </InlineField>
        </>
      )}

      {isPassword && (
        <>
          <InlineField
            label="Email"
            labelWidth={LABEL_WIDTH}
            tooltip="Baserow account email. Used to sign in and obtain a JWT (auto-refreshed)."
            required
          >
            <Input
              width={INPUT_WIDTH}
              name="email"
              placeholder="you@example.com"
              value={jsonData.email ?? ''}
              onChange={onEmailChange}
            />
          </InlineField>

          <InlineField
            label="Password"
            labelWidth={LABEL_WIDTH}
            tooltip="Baserow account password. Stored encrypted and never sent to the browser."
            required
          >
            <SecretInput
              width={INPUT_WIDTH}
              isConfigured={Boolean(secureJsonFields?.password)}
              value={secureJsonData?.password ?? ''}
              placeholder="Enter Baserow password"
              onReset={onResetPassword}
              onChange={onPasswordChange}
            />
          </InlineField>

          <InlineField
            label="Default Database ID"
            labelWidth={LABEL_WIDTH}
            tooltip="Optional. Pre-select a database to list tables from. With email & password you can also choose the database in the query editor."
          >
            <Input
              width={INPUT_WIDTH}
              name="databaseId"
              placeholder="optional"
              value={jsonData.databaseId ?? ''}
              onChange={onDatabaseIdChange}
            />
          </InlineField>
        </>
      )}
    </FieldSet>
  );
}

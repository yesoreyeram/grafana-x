import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet, RadioButtonGroup } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';

import { AuthMode, PocketBaseDataSourceOptions, PocketBaseSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<PocketBaseDataSourceOptions, PocketBaseSecureJsonData> {}

const LABEL_WIDTH = 22;
const INPUT_WIDTH = 50;

const POCKETBASE_DEFAULT_URL = 'http://127.0.0.1:8090';

const AUTH_MODE_OPTIONS: Array<SelectableValue<AuthMode>> = [
  { label: 'Superuser', value: 'superuser', description: 'Authenticate against the _superusers collection (full read access).' },
  { label: 'User', value: 'user', description: 'Authenticate against a regular auth collection (constrained by API rules).' },
  { label: 'Token', value: 'token', description: 'Use a pre-issued auth token verbatim (no password exchange).' },
];

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;
  const authMode: AuthMode = jsonData.authMode ?? 'superuser';

  const onUrlChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, url: event.target.value } });
  };

  const onAuthModeChange = (value: AuthMode) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, authMode: value } });
  };

  const onIdentityChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, identity: event.target.value } });
  };

  const onAuthCollectionChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, authCollection: event.target.value } });
  };

  const onPasswordChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, secureJsonData: { ...secureJsonData, password: event.target.value } });
  };

  const onResetPassword = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, password: false },
      secureJsonData: { ...secureJsonData, password: '' },
    });
  };

  const onTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, secureJsonData: { ...secureJsonData, authToken: event.target.value } });
  };

  const onResetToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, authToken: false },
      secureJsonData: { ...secureJsonData, authToken: '' },
    });
  };

  const usesPassword = authMode === 'superuser' || authMode === 'user';

  return (
    <FieldSet label="PocketBase Connection">
      <InlineField
        label="URL"
        labelWidth={LABEL_WIDTH}
        tooltip="PocketBase base URL, e.g. http://127.0.0.1:8090 (no trailing /api). For a self-hosted instance use https://<your-host>."
        required
      >
        <Input
          width={INPUT_WIDTH}
          name="url"
          placeholder={POCKETBASE_DEFAULT_URL}
          value={jsonData.url ?? ''}
          onChange={onUrlChange}
        />
      </InlineField>

      <InlineField
        label="Auth mode"
        labelWidth={LABEL_WIDTH}
        tooltip="How to authenticate. Superuser lists collections and reads every record. User auth is constrained by each collection's API rules. Token uses a pre-issued token verbatim."
      >
        <RadioButtonGroup<AuthMode> options={AUTH_MODE_OPTIONS} value={authMode} onChange={onAuthModeChange} />
      </InlineField>

      {usesPassword && (
        <>
          <InlineField
            label="Identity (email)"
            labelWidth={LABEL_WIDTH}
            tooltip="The superuser or user email (or username) used for password authentication."
            required
          >
            <Input
              width={INPUT_WIDTH}
              name="identity"
              placeholder="admin@example.com"
              value={jsonData.identity ?? ''}
              onChange={onIdentityChange}
            />
          </InlineField>

          {authMode === 'user' && (
            <InlineField
              label="Auth collection"
              labelWidth={LABEL_WIDTH}
              tooltip="The auth collection to authenticate against in user mode. Defaults to `users`."
            >
              <Input
                width={INPUT_WIDTH}
                name="authCollection"
                placeholder="users"
                value={jsonData.authCollection ?? ''}
                onChange={onAuthCollectionChange}
              />
            </InlineField>
          )}

          <InlineField
            label="Password"
            labelWidth={LABEL_WIDTH}
            tooltip="The account password. Exchanged for an auth token server-side and stored as a secret."
            required
          >
            <SecretInput
              width={INPUT_WIDTH}
              isConfigured={Boolean(secureJsonFields?.password)}
              value={secureJsonData?.password ?? ''}
              placeholder="Enter password"
              onReset={onResetPassword}
              onChange={onPasswordChange}
            />
          </InlineField>
        </>
      )}

      {authMode === 'token' && (
        <InlineField
          label="Auth token"
          labelWidth={LABEL_WIDTH}
          tooltip="A pre-issued PocketBase auth token (for example an impersonate token), sent verbatim in the Authorization header. Stored as a secret."
          required
        >
          <SecretInput
            width={INPUT_WIDTH}
            isConfigured={Boolean(secureJsonFields?.authToken)}
            value={secureJsonData?.authToken ?? ''}
            placeholder="Enter auth token"
            onReset={onResetToken}
            onChange={onTokenChange}
          />
        </InlineField>
      )}
    </FieldSet>
  );
}

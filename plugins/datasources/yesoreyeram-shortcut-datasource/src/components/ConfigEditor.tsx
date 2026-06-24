import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { ShortcutDataSourceOptions, ShortcutSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<ShortcutDataSourceOptions, ShortcutSecureJsonData> {}

const LABEL_WIDTH = 24;
const INPUT_WIDTH = 50;

const SHORTCUT_API_URL = 'https://api.app.shortcut.com';

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, baseURL: event.target.value },
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

  return (
    <FieldSet label="Shortcut Connection">
      <InlineField
        label="API URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Shortcut API host. Defaults to https://api.app.shortcut.com. Override only to point at a proxy; the /api/v3 path is added automatically."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder={SHORTCUT_API_URL}
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>

      <InlineField
        label="API Token"
        labelWidth={LABEL_WIDTH}
        tooltip="Shortcut personal API token. Create one at Shortcut Settings > API Tokens (https://app.shortcut.com/settings/account/api-tokens). Sent as the Shortcut-Token header."
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiToken)}
          value={secureJsonData?.apiToken ?? ''}
          placeholder="Shortcut API token"
          onReset={onResetApiToken}
          onChange={onApiTokenChange}
        />
      </InlineField>
    </FieldSet>
  );
}

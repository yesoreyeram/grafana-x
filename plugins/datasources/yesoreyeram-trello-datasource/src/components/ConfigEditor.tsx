import React, { ChangeEvent } from 'react';
import { InlineField, SecretInput, FieldSet, InlineFieldRow } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { TrelloDataSourceOptions, TrelloSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<TrelloDataSourceOptions, TrelloSecureJsonData> {}

const LABEL_WIDTH = 20;
const INPUT_WIDTH = 50;

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { secureJsonFields, secureJsonData } = options;

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
    <FieldSet label="Trello Connection">
      <InlineFieldRow>
        <InlineField
          label="API Key"
          labelWidth={LABEL_WIDTH}
          tooltip="Trello API key. Obtain one at https://trello.com/app-key"
        >
          <SecretInput
            width={INPUT_WIDTH}
            isConfigured={Boolean(secureJsonFields?.apiKey)}
            value={secureJsonData?.apiKey ?? ''}
            placeholder="your-trello-api-key"
            onReset={onResetApiKey}
            onChange={onApiKeyChange}
          />
        </InlineField>
      </InlineFieldRow>

      <InlineFieldRow>
        <InlineField
          label="API Token"
          labelWidth={LABEL_WIDTH}
          tooltip="Trello API token. Generate one from your API key at https://trello.com/app-key"
        >
          <SecretInput
            width={INPUT_WIDTH}
            isConfigured={Boolean(secureJsonFields?.apiToken)}
            value={secureJsonData?.apiToken ?? ''}
            placeholder="your-trello-api-token"
            onReset={onResetApiToken}
            onChange={onApiTokenChange}
          />
        </InlineField>
      </InlineFieldRow>

      <InlineFieldRow>
        <InlineField label="" labelWidth={LABEL_WIDTH}>
          <div style={{ fontSize: '12px', color: 'var(--text-secondary)', maxWidth: 600 }}>
            To obtain your Trello API credentials, visit{' '}
            <a href="https://trello.com/app-key" target="_blank" rel="noreferrer">
              https://trello.com/app-key
            </a>
            . The <strong>API Key</strong> is shown on that page. Generate an{' '}
            <strong>API Token</strong> by clicking the Token link under the API key.
            Both values are required and are sent as query parameters to authenticate
            every request.
          </div>
        </InlineField>
      </InlineFieldRow>
    </FieldSet>
  );
}

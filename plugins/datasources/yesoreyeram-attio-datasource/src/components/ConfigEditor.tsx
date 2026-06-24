import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { AttioDataSourceOptions, AttioSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<AttioDataSourceOptions, AttioSecureJsonData> {}

const LABEL_WIDTH = 22;
const INPUT_WIDTH = 50;

const ATTIO_API_URL = 'https://api.attio.com';

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, baseURL: event.target.value },
    });
  };

  const onDefaultObjectChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, defaultObjectId: event.target.value },
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

  return (
    <FieldSet label="Attio Connection">
      <InlineField
        label="API Token"
        labelWidth={LABEL_WIDTH}
        tooltip="Attio workspace access token, sent as 'Authorization: Bearer <token>'. Generate one in Attio under Settings > Developers > API keys."
        required
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiToken)}
          value={secureJsonData?.apiToken ?? ''}
          placeholder="Enter Attio API token"
          onReset={onResetToken}
          onChange={onTokenChange}
        />
      </InlineField>

      <InlineField
        label="API URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Root URL of the Attio API. Defaults to https://api.attio.com. Override only to point at a proxy."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder={ATTIO_API_URL}
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>

      <InlineField
        label="Default Object"
        labelWidth={LABEL_WIDTH}
        tooltip="Optional. An Attio object api_slug (e.g. people, companies, deals). When set, the query editor selects this object by default."
      >
        <Input
          width={INPUT_WIDTH}
          name="defaultObjectId"
          placeholder="people (optional)"
          value={jsonData.defaultObjectId ?? ''}
          onChange={onDefaultObjectChange}
        />
      </InlineField>
    </FieldSet>
  );
}

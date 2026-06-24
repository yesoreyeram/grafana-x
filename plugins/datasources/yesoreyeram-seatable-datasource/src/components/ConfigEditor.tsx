import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { SeaTableDataSourceOptions, SeaTableSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<SeaTableDataSourceOptions, SeaTableSecureJsonData> {}

const LABEL_WIDTH = 22;
const INPUT_WIDTH = 50;

const SEATABLE_DEFAULT_URL = 'https://cloud.seatable.io';

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onServerURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, serverURL: event.target.value },
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
    <FieldSet label="SeaTable Connection">
      <InlineField
        label="Server URL"
        labelWidth={LABEL_WIDTH}
        tooltip="SeaTable server URL. Use https://cloud.seatable.io for SeaTable Cloud, or your self-hosted server URL."
      >
        <Input
          width={INPUT_WIDTH}
          name="serverURL"
          placeholder={SEATABLE_DEFAULT_URL}
          value={jsonData.serverURL ?? ''}
          onChange={onServerURLChange}
        />
      </InlineField>

      <InlineField
        label="Base API Token"
        labelWidth={LABEL_WIDTH}
        tooltip="SeaTable Base API Token (created from a base's API Tokens panel). It is scoped to a single base. The backend exchanges it for a short-lived Base-Token and the base's dtable_uuid; data calls use that Base-Token. Stored encrypted; never sent to the browser."
        required
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiToken)}
          value={secureJsonData?.apiToken ?? ''}
          placeholder="Enter SeaTable Base API Token"
          onReset={onResetToken}
          onChange={onTokenChange}
        />
      </InlineField>
    </FieldSet>
  );
}

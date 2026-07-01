import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { CodaDataSourceOptions, CodaSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<CodaDataSourceOptions, CodaSecureJsonData> {}

const LABEL_WIDTH = 22;
const INPUT_WIDTH = 50;

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onDocIdChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, docId: event.target.value },
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
    <FieldSet label="Coda Connection">
      <InlineField
        label="API Token"
        labelWidth={LABEL_WIDTH}
        tooltip="Coda API token, sent as 'Authorization: Bearer <token>'. Create one at coda.io/account."
        required
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiToken)}
          value={secureJsonData?.apiToken ?? ''}
          placeholder="Enter Coda API token"
          onReset={onResetToken}
          onChange={onTokenChange}
        />
      </InlineField>

      <InlineField
        label="Default Doc ID"
        labelWidth={LABEL_WIDTH}
        tooltip="Optional. Coda doc id. When set, the query editor lists this doc's tables directly. Otherwise you can pick a doc in the query editor."
      >
        <Input
          width={INPUT_WIDTH}
          name="docId"
          placeholder="coda-doc-id (optional)"
          value={jsonData.docId ?? ''}
          onChange={onDocIdChange}
        />
      </InlineField>
    </FieldSet>
  );
}

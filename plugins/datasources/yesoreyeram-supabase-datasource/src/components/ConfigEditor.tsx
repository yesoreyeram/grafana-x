import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { SupabaseDataSourceOptions, SupabaseSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<SupabaseDataSourceOptions, SupabaseSecureJsonData> {}

const LABEL_WIDTH = 22;
const INPUT_WIDTH = 50;

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onApiUrlChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, apiUrl: event.target.value },
    });
  };

  const onSchemaChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, schema: event.target.value },
    });
  };

  const onServiceKeyChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: { ...secureJsonData, serviceKey: event.target.value },
    });
  };

  const onResetServiceKey = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, serviceKey: false },
      secureJsonData: { ...secureJsonData, serviceKey: '' },
    });
  };

  return (
    <FieldSet label="Supabase Connection">
      <InlineField
        label="Project URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Supabase PostgREST endpoint. e.g. https://xxx.supabase.co/rest/v1"
        required
      >
        <Input
          width={INPUT_WIDTH}
          name="apiUrl"
          placeholder="https://xxx.supabase.co/rest/v1"
          value={jsonData.apiUrl ?? ''}
          onChange={onApiUrlChange}
        />
      </InlineField>

      <InlineField
        label="Service Role Key"
        labelWidth={LABEL_WIDTH}
        tooltip="Supabase anon or service_role key. Sent as both 'apikey' header and 'Authorization: Bearer <key>'. Stored encrypted; never sent to the browser."
        required
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.serviceKey)}
          value={secureJsonData?.serviceKey ?? ''}
          placeholder="Enter Supabase anon / service_role key"
          onReset={onResetServiceKey}
          onChange={onServiceKeyChange}
        />
      </InlineField>

      <InlineField
        label="Schema"
        labelWidth={LABEL_WIDTH}
        tooltip="Optional Postgres schema, sent via the Accept-Profile header. Leave empty to use the PostgREST default (public)."
      >
        <Input
          width={INPUT_WIDTH}
          name="schema"
          placeholder="public"
          value={jsonData.schema ?? ''}
          onChange={onSchemaChange}
        />
      </InlineField>
    </FieldSet>
  );
}

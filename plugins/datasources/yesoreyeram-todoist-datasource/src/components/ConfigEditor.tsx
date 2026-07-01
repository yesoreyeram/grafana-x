import React, { ChangeEvent } from 'react';
import { InlineField, SecretInput, FieldSet } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { TodoistDataSourceOptions, TodoistSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<TodoistDataSourceOptions, TodoistSecureJsonData> {}

const LABEL_WIDTH = 24;
const INPUT_WIDTH = 50;

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { secureJsonFields, secureJsonData } = options;

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
    <FieldSet label="Todoist Connection">
      <InlineField
        label="API Token"
        labelWidth={LABEL_WIDTH}
        tooltip="Your Todoist API token. Create one at Todoist Settings > Integrations > Developer."
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiToken)}
          value={secureJsonData?.apiToken ?? ''}
          placeholder="0123456789abcdef0123456789abcdef01234567"
          onReset={onResetApiToken}
          onChange={onApiTokenChange}
        />
      </InlineField>
    </FieldSet>
  );
}

import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet, RadioButtonGroup } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { IntercomDataSourceOptions, IntercomSecureJsonData, IntercomRegion, REGION_OPTIONS } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<IntercomDataSourceOptions, IntercomSecureJsonData> {}

const LABEL_WIDTH = 24;
const INPUT_WIDTH = 50;

const INTERCOM_API_URL = 'https://api.intercom.io';
const DEFAULT_INTERCOM_VERSION = '2.11';

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;
  const region: IntercomRegion = jsonData.region ?? 'us';

  const onRegionChange = (value: IntercomRegion) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, region: value } });
  };

  const onBaseURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, baseURL: event.target.value } });
  };

  const onIntercomVersionChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, intercomVersion: event.target.value } });
  };

  const onTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, secureJsonData: { ...secureJsonData, apiToken: event.target.value } });
  };

  const onResetToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, apiToken: false },
      secureJsonData: { ...secureJsonData, apiToken: '' },
    });
  };

  return (
    <FieldSet label="Intercom Connection">
      <InlineField
        label="Region"
        labelWidth={LABEL_WIDTH}
        tooltip="Intercom data residency region. Determines the API host unless an explicit API URL is set below."
      >
        <RadioButtonGroup<IntercomRegion> options={REGION_OPTIONS} value={region} onChange={onRegionChange} />
      </InlineField>

      <InlineField
        label="API URL"
        labelWidth={LABEL_WIDTH}
        tooltip="Optional. Overrides the region-derived host. Use https://api.intercom.io (US), https://api.eu.intercom.io (EU) or https://api.au.intercom.io (AU), or a proxy."
      >
        <Input
          width={INPUT_WIDTH}
          name="baseURL"
          placeholder={INTERCOM_API_URL}
          value={jsonData.baseURL ?? ''}
          onChange={onBaseURLChange}
        />
      </InlineField>

      <InlineField
        label="Intercom-Version"
        labelWidth={LABEL_WIDTH}
        tooltip="Value of the Intercom-Version header sent with every request."
      >
        <Input
          width={INPUT_WIDTH}
          name="intercomVersion"
          placeholder={DEFAULT_INTERCOM_VERSION}
          value={jsonData.intercomVersion ?? ''}
          onChange={onIntercomVersionChange}
        />
      </InlineField>

      <InlineField
        label="Access Token"
        labelWidth={LABEL_WIDTH}
        tooltip="Intercom access token, sent as the Authorization: Bearer header. Create one in the Intercom Developer Hub or app settings."
      >
        <SecretInput
          width={INPUT_WIDTH}
          isConfigured={Boolean(secureJsonFields?.apiToken)}
          value={secureJsonData?.apiToken ?? ''}
          placeholder="dG9rZW46..."
          onReset={onResetToken}
          onChange={onTokenChange}
        />
      </InlineField>
    </FieldSet>
  );
}

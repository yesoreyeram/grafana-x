import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet, RadioButtonGroup } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';
import { PipedriveAuthMethod, PipedriveDataSourceOptions, PipedriveSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<PipedriveDataSourceOptions, PipedriveSecureJsonData> {}

const LABEL_WIDTH = 24;
const INPUT_WIDTH = 50;

const AUTH_OPTIONS: Array<SelectableValue<PipedriveAuthMethod>> = [
  { label: 'API token', value: 'apiToken', description: 'Pipedrive API token, sent as the api_token query parameter' },
  { label: 'OAuth token', value: 'oauth', description: 'Pipedrive OAuth2 access token, sent as an Authorization: Bearer header' },
];

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;
  const authMethod: PipedriveAuthMethod = jsonData.authMethod ?? 'apiToken';

  const onCompanyDomainChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, companyDomain: event.target.value } });
  };
  const onAuthMethodChange = (value: PipedriveAuthMethod) => {
    onOptionsChange({ ...options, jsonData: { ...jsonData, authMethod: value } });
  };
  const onApiTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, secureJsonData: { ...secureJsonData, apiToken: event.target.value } });
  };
  const onResetApiToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, apiToken: false },
      secureJsonData: { ...secureJsonData, apiToken: '' },
    });
  };
  const onOAuthTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({ ...options, secureJsonData: { ...secureJsonData, oauthToken: event.target.value } });
  };
  const onResetOAuthToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, oauthToken: false },
      secureJsonData: { ...secureJsonData, oauthToken: '' },
    });
  };

  return (
    <FieldSet label="Pipedrive Connection">
      <InlineField
        label="Company Domain"
        labelWidth={LABEL_WIDTH}
        tooltip="Your Pipedrive company subdomain (e.g. 'mycompany' from mycompany.pipedrive.com). URL is built as https://{domain}.pipedrive.com/api/v1"
      >
        <Input
          width={INPUT_WIDTH}
          name="companyDomain"
          placeholder="mycompany"
          value={jsonData.companyDomain ?? ''}
          onChange={onCompanyDomainChange}
        />
      </InlineField>
      <InlineField label="Authentication" labelWidth={LABEL_WIDTH} tooltip="Pipedrive authentication method.">
        <RadioButtonGroup<PipedriveAuthMethod> options={AUTH_OPTIONS} value={authMethod} onChange={onAuthMethodChange} />
      </InlineField>
      {authMethod === 'apiToken' && (
        <InlineField
          label="API Token"
          labelWidth={LABEL_WIDTH}
          tooltip="Pipedrive API token. Generate at Settings > Personal preferences > API in Pipedrive. Sent as the api_token query parameter."
        >
          <SecretInput
            width={INPUT_WIDTH}
            isConfigured={Boolean(secureJsonFields?.apiToken)}
            value={secureJsonData?.apiToken ?? ''}
            placeholder="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
            onReset={onResetApiToken}
            onChange={onApiTokenChange}
          />
        </InlineField>
      )}
      {authMethod === 'oauth' && (
        <InlineField
          label="OAuth Token"
          labelWidth={LABEL_WIDTH}
          tooltip="Pipedrive OAuth2 access token. Sent as an Authorization: Bearer header."
        >
          <SecretInput
            width={INPUT_WIDTH}
            isConfigured={Boolean(secureJsonFields?.oauthToken)}
            value={secureJsonData?.oauthToken ?? ''}
            placeholder="access token"
            onReset={onResetOAuthToken}
            onChange={onOAuthTokenChange}
          />
        </InlineField>
      )}
    </FieldSet>
  );
}

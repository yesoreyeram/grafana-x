import React from 'react';
import { SelectableValue } from '@grafana/data';
import { InlineField, InlineFieldRow, Input, Select, Switch } from '@grafana/ui';

import { EncodingModel, PropMap } from '../../types';

const LABEL = 11;

const SCALE_TYPES: Array<SelectableValue<string>> = [
  '',
  'linear',
  'log',
  'pow',
  'sqrt',
  'symlog',
  'time',
  'utc',
  'ordinal',
  'band',
  'point',
  'quantile',
  'quantize',
  'threshold',
  'bin-ordinal',
].map((v) => ({ label: v === '' ? '(default)' : v, value: v }));

const TRISTATE: Array<SelectableValue<string>> = [
  { label: '(default)', value: '' },
  { label: 'on', value: 'true' },
  { label: 'off', value: 'false' },
];

const LEGEND_ORIENT: Array<SelectableValue<string>> = [
  '',
  'left',
  'right',
  'top',
  'bottom',
  'top-left',
  'top-right',
  'bottom-left',
  'bottom-right',
].map((v) => ({ label: v === '' ? '(default)' : v, value: v }));

const AXIS_ORIENT: Array<SelectableValue<string>> = ['', 'top', 'bottom', 'left', 'right'].map((v) => ({
  label: v === '' ? '(default)' : v,
  value: v,
}));

function withKey(obj: PropMap | null | undefined, key: string, value: unknown): PropMap | undefined {
  const next: PropMap = obj && typeof obj === 'object' ? { ...obj } : {};
  if (value === undefined || value === '' || value === null) {
    delete next[key];
  } else {
    next[key] = value;
  }
  return Object.keys(next).length > 0 ? next : undefined;
}

function str(obj: PropMap | null | undefined, key: string): string {
  const v = obj && typeof obj === 'object' ? obj[key] : undefined;
  return typeof v === 'string' ? v : '';
}

function tri(obj: PropMap | null | undefined, key: string): string {
  const v = obj && typeof obj === 'object' ? obj[key] : undefined;
  return v === true ? 'true' : v === false ? 'false' : '';
}

interface Props {
  enc: EncodingModel;
  onChange: (patch: Partial<EncodingModel>) => void;
}

/** Typed editors for an encoding's scale, axis and legend (no raw JSON). */
export function ChannelStyleEditor({ enc, onChange }: Props) {
  const axisHidden = enc.axis === null;
  const legendHidden = enc.legend === null;

  return (
    <>
      <InlineFieldRow>
        <InlineField label="Scale type" labelWidth={LABEL}>
          <Select
            width={18}
            options={SCALE_TYPES}
            value={str(enc.scale, 'type')}
            onChange={(v) => onChange({ scale: withKey(enc.scale, 'type', v.value) })}
          />
        </InlineField>
        <InlineField label="Zero">
          <Select
            width={14}
            options={TRISTATE}
            value={tri(enc.scale, 'zero')}
            onChange={(v) => onChange({ scale: withKey(enc.scale, 'zero', v.value === '' ? undefined : v.value === 'true') })}
          />
        </InlineField>
      </InlineFieldRow>

      <InlineFieldRow>
        <InlineField label="Scheme" labelWidth={LABEL} tooltip="Color scheme for this channel (overrides the panel palette).">
          <Input
            width={18}
            value={str(enc.scale, 'scheme')}
            placeholder="(theme)"
            onChange={(e) => onChange({ scale: withKey(enc.scale, 'scheme', e.currentTarget.value || undefined) })}
          />
        </InlineField>
        <InlineField label="Nice">
          <Switch
            value={Boolean(enc.scale?.nice)}
            onChange={(e) => onChange({ scale: withKey(enc.scale, 'nice', e.currentTarget.checked || undefined) })}
          />
        </InlineField>
        <InlineField label="Reverse">
          <Switch
            value={Boolean(enc.scale?.reverse)}
            onChange={(e) => onChange({ scale: withKey(enc.scale, 'reverse', e.currentTarget.checked || undefined) })}
          />
        </InlineField>
      </InlineFieldRow>

      <InlineFieldRow>
        <InlineField label="Hide axis" labelWidth={LABEL}>
          <Switch value={axisHidden} onChange={(e) => onChange({ axis: e.currentTarget.checked ? null : undefined })} />
        </InlineField>
        {!axisHidden && (
          <InlineField label="Axis grid">
            <Select
              width={14}
              options={TRISTATE}
              value={tri(enc.axis, 'grid')}
              onChange={(v) => onChange({ axis: withKey(enc.axis, 'grid', v.value === '' ? undefined : v.value === 'true') })}
            />
          </InlineField>
        )}
      </InlineFieldRow>

      {!axisHidden && (
        <InlineFieldRow>
          <InlineField label="Axis title" labelWidth={LABEL}>
            <Input
              width={18}
              value={str(enc.axis, 'title')}
              onChange={(e) => onChange({ axis: withKey(enc.axis, 'title', e.currentTarget.value || undefined) })}
            />
          </InlineField>
          <InlineField label="Orient">
            <Select
              width={14}
              options={AXIS_ORIENT}
              value={str(enc.axis, 'orient')}
              onChange={(v) => onChange({ axis: withKey(enc.axis, 'orient', v.value) })}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      <InlineFieldRow>
        <InlineField label="Hide legend" labelWidth={LABEL}>
          <Switch value={legendHidden} onChange={(e) => onChange({ legend: e.currentTarget.checked ? null : undefined })} />
        </InlineField>
        {!legendHidden && (
          <InlineField label="Legend orient">
            <Select
              width={18}
              options={LEGEND_ORIENT}
              value={str(enc.legend, 'orient')}
              onChange={(v) => onChange({ legend: withKey(enc.legend, 'orient', v.value) })}
            />
          </InlineField>
        )}
      </InlineFieldRow>
    </>
  );
}

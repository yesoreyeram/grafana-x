import React, { useState } from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2, SelectableValue } from '@grafana/data';
import { Button, IconButton, InlineField, InlineFieldRow, Input, Select, Switch, useStyles2 } from '@grafana/ui';

import { AggregateOp, EncodingChannelName, EncodingModel, StackMode, VegaLiteFieldType } from '../../types';
import { ChannelStyleEditor } from './ChannelStyleEditor';
import { AGGREGATE_OPTIONS, CHANNEL_OPTIONS, STACK_OPTIONS, TIME_UNIT_OPTIONS, TYPE_OPTIONS } from './options';

interface Props {
  encodings: EncodingModel[];
  fieldOptions: Array<SelectableValue<string>>;
  fieldTypes: Record<string, VegaLiteFieldType>;
  onChange: (encodings: EncodingModel[]) => void;
}

let idCounter = 0;
function newId(): string {
  idCounter += 1;
  return `enc-${Date.now()}-${idCounter}`;
}

const LABEL_WIDTH = 11;

function summary(enc: EncodingModel): string {
  if (enc.field) {
    return `${enc.channel}: ${enc.field}${enc.aggregate ? ` (${enc.aggregate})` : ''}`;
  }
  if (enc.aggregate === 'count') {
    return `${enc.channel}: count()`;
  }
  if (enc.value) {
    return `${enc.channel} = ${enc.value}`;
  }
  return `${enc.channel}: (unset)`;
}

interface RowProps {
  enc: EncodingModel;
  open: boolean;
  fieldOptions: Array<SelectableValue<string>>;
  fieldTypes: Record<string, VegaLiteFieldType>;
  onToggle: () => void;
  onChange: (patch: Partial<EncodingModel>) => void;
  onRemove: () => void;
}

function EncodingRow({ enc, open, fieldOptions, fieldTypes, onToggle, onChange, onRemove }: RowProps) {
  const styles = useStyles2(getStyles);
  const disabled = enc.enabled === false;

  return (
    <div className={styles.row}>
      <div className={styles.header}>
        <IconButton name={open ? 'angle-down' : 'angle-right'} aria-label="Expand" onClick={onToggle} />
        <button type="button" className={styles.summary} onClick={onToggle}>
          <span className={disabled ? styles.disabled : undefined}>{summary(enc)}</span>
        </button>
        <Switch
          value={enc.enabled !== false}
          onChange={(e) => onChange({ enabled: e.currentTarget.checked })}
        />
        <IconButton name="trash-alt" aria-label="Remove encoding" onClick={onRemove} />
      </div>

      {open && (
        <div className={styles.body}>
          <InlineField label="Channel" labelWidth={LABEL_WIDTH} grow>
            <Select<EncodingChannelName>
              options={CHANNEL_OPTIONS}
              value={enc.channel}
              onChange={(v) => v.value && onChange({ channel: v.value })}
            />
          </InlineField>

          <InlineField label="Field" labelWidth={LABEL_WIDTH} grow>
            <Select<string>
              options={fieldOptions}
              value={enc.field ?? null}
              isClearable
              allowCustomValue
              placeholder="Select a field"
              onChange={(v) => {
                const field = v?.value;
                const nextType = field && !enc.type ? fieldTypes[field] : enc.type;
                onChange({ field: field ?? undefined, type: nextType });
              }}
            />
          </InlineField>

          <InlineFieldRow>
            <InlineField label="Type" labelWidth={LABEL_WIDTH}>
              <Select<VegaLiteFieldType>
                options={TYPE_OPTIONS}
                value={enc.type ?? null}
                isClearable
                width={18}
                placeholder="auto"
                onChange={(v) => onChange({ type: v?.value })}
              />
            </InlineField>
            <InlineField label="Aggregate">
              <Select<AggregateOp | ''>
                options={AGGREGATE_OPTIONS}
                value={enc.aggregate ?? ''}
                width={16}
                onChange={(v) => onChange({ aggregate: v.value ? (v.value as AggregateOp) : undefined })}
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField label="Time unit" labelWidth={LABEL_WIDTH}>
              <Select<string>
                options={TIME_UNIT_OPTIONS}
                value={enc.timeUnit ?? ''}
                width={20}
                onChange={(v) => onChange({ timeUnit: v.value || undefined })}
              />
            </InlineField>
            <InlineField label="Bin">
              <Switch value={Boolean(enc.bin)} onChange={(e) => onChange({ bin: e.currentTarget.checked })} />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField label="Stack" labelWidth={LABEL_WIDTH}>
              <Select<StackMode>
                options={STACK_OPTIONS}
                value={enc.stack ?? null}
                isClearable
                width={16}
                placeholder="auto"
                onChange={(v) => onChange({ stack: v?.value })}
              />
            </InlineField>
            <InlineField label="Sort">
              <Input
                width={18}
                value={enc.sort ?? ''}
                placeholder="ascending / -x"
                onChange={(e) => onChange({ sort: e.currentTarget.value || undefined })}
              />
            </InlineField>
          </InlineFieldRow>

          <InlineField label="Title" labelWidth={LABEL_WIDTH} grow>
            <Input
              value={enc.title ?? ''}
              placeholder="Axis / legend title"
              onChange={(e) => onChange({ title: e.currentTarget.value || undefined })}
            />
          </InlineField>

          <InlineFieldRow>
            <InlineField label="Format" labelWidth={LABEL_WIDTH}>
              <Input
                width={16}
                value={enc.format ?? ''}
                placeholder="d3 format"
                onChange={(e) => onChange({ format: e.currentTarget.value || undefined })}
              />
            </InlineField>
            <InlineField label="Value" tooltip="Constant value instead of a field (e.g. a fixed color)">
              <Input
                width={16}
                value={enc.value ?? ''}
                placeholder='"red" / 5'
                onChange={(e) => onChange({ value: e.currentTarget.value || undefined })}
              />
            </InlineField>
          </InlineFieldRow>

          <ChannelStyleEditor enc={enc} onChange={onChange} />
        </div>
      )}
    </div>
  );
}

export function EncodingEditor({ encodings, fieldOptions, fieldTypes, onChange }: Props) {
  const styles = useStyles2(getStyles);
  const [openIds, setOpenIds] = useState<Set<string>>(new Set());

  const toggle = (id: string) => {
    setOpenIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const add = () => {
    const id = newId();
    onChange([...encodings, { id, channel: 'x', enabled: true }]);
    setOpenIds((prev) => new Set(prev).add(id));
  };

  const update = (id: string, patch: Partial<EncodingModel>) =>
    onChange(encodings.map((e) => (e.id === id ? { ...e, ...patch } : e)));

  const remove = (id: string) => onChange(encodings.filter((e) => e.id !== id));

  return (
    <div className={styles.list}>
      {encodings.map((enc) => (
        <EncodingRow
          key={enc.id}
          enc={enc}
          open={openIds.has(enc.id)}
          fieldOptions={fieldOptions}
          fieldTypes={fieldTypes}
          onToggle={() => toggle(enc.id)}
          onChange={(patch) => update(enc.id, patch)}
          onRemove={() => remove(enc.id)}
        />
      ))}
      <Button variant="secondary" size="sm" icon="plus" onClick={add}>
        Add encoding
      </Button>
    </div>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  list: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  row: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
  }),
  header: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    padding: theme.spacing(0.5, 1),
  }),
  summary: css({
    flex: 1,
    textAlign: 'left',
    background: 'transparent',
    border: 'none',
    cursor: 'pointer',
    color: theme.colors.text.primary,
    fontSize: theme.typography.bodySmall.fontSize,
    padding: 0,
  }),
  disabled: css({ textDecoration: 'line-through', color: theme.colors.text.disabled }),
  body: css({ padding: theme.spacing(1), borderTop: `1px solid ${theme.colors.border.weak}` }),
});

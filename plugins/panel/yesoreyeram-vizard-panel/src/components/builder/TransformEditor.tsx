import React, { useEffect, useState } from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2, SelectableValue } from '@grafana/data';
import { Button, IconButton, InlineField, Input, MultiSelect, Select, Switch, useStyles2 } from '@grafana/ui';

import { TransformKind, TransformModel } from '../../types';
import { JsonInput } from './JsonInput';
import { TRANSFORM_OPTIONS, TRANSFORM_TEMPLATES } from './options';
import { getTransformSpec, parseTransformObject, serializeTransform, TField, TValues } from './transformSchema';

interface Props {
  transforms: TransformModel[];
  fieldOptions: Array<SelectableValue<string>>;
  onChange: (transforms: TransformModel[]) => void;
}

let idCounter = 0;
function newId(): string {
  idCounter += 1;
  return `tf-${Date.now()}-${idCounter}`;
}

function isBlankJson(json: string): boolean {
  const t = json.trim();
  return t === '' || t === '{}';
}

function asString(v: string | string[] | undefined): string {
  return typeof v === 'string' ? v : '';
}

function asArray(v: string | string[] | undefined): string[] {
  return Array.isArray(v) ? v : [];
}

interface FieldProps {
  desc: TField;
  values: TValues;
  fieldOptions: Array<SelectableValue<string>>;
  onSet: (key: string, value: string | string[], commit: boolean) => void;
}

function TransformField({ desc, values, fieldOptions, onSet }: FieldProps) {
  if (desc.kind === 'fields') {
    const selected = asArray(values[desc.key]).map((v) => fieldOptions.find((o) => o.value === v) ?? { label: v, value: v });
    return (
      <MultiSelect
        options={fieldOptions}
        value={selected}
        allowCustomValue
        placeholder="Select fields"
        onChange={(vs) => onSet(desc.key, vs.map((x) => x.value).filter((x): x is string => Boolean(x)), true)}
      />
    );
  }

  if (desc.kind === 'field') {
    return (
      <Select<string>
        options={fieldOptions}
        value={asString(values[desc.key]) || null}
        isClearable
        allowCustomValue
        placeholder="Select a field"
        onChange={(v) => onSet(desc.key, v?.value ?? '', true)}
      />
    );
  }

  if (desc.kind === 'select') {
    return (
      <Select<string>
        options={desc.options ?? []}
        value={asString(values[desc.key]) || null}
        isClearable
        placeholder="Select"
        onChange={(v) => onSet(desc.key, v?.value ?? '', true)}
      />
    );
  }

  // text / expr / number: commit on blur so typing stays smooth
  return (
    <Input
      type={desc.kind === 'number' ? 'number' : 'text'}
      value={asString(values[desc.key])}
      placeholder={desc.placeholder}
      spellCheck={false}
      onChange={(e) => onSet(desc.key, e.currentTarget.value, false)}
      onBlur={(e) => onSet(desc.key, e.currentTarget.value, true)}
    />
  );
}

interface StructuredProps {
  kind: TransformKind;
  json: string;
  fieldOptions: Array<SelectableValue<string>>;
  onJsonChange: (json: string) => void;
}

function StructuredFields({ kind, json, fieldOptions, onJsonChange }: StructuredProps) {
  const [values, setValues] = useState<TValues>(() => getTransformSpec(kind).extract(parseTransformObject(json)));

  useEffect(() => {
    setValues(getTransformSpec(kind).extract(parseTransformObject(json)));
  }, [kind, json]);

  const onSet = (key: string, value: string | string[], commit: boolean) => {
    const next = { ...values, [key]: value };
    setValues(next);
    if (commit) {
      onJsonChange(serializeTransform(getTransformSpec(kind).build(next)));
    }
  };

  return (
    <>
      {getTransformSpec(kind).fields.map((desc) => (
        <InlineField key={desc.key} label={desc.label} labelWidth={16} grow tooltip={desc.tooltip}>
          <TransformField desc={desc} values={values} fieldOptions={fieldOptions} onSet={onSet} />
        </InlineField>
      ))}
    </>
  );
}

export function TransformEditor({ transforms, fieldOptions, onChange }: Props) {
  const styles = useStyles2(getStyles);

  const add = () => {
    const spec = getTransformSpec('filter');
    const json = serializeTransform(spec.build(spec.extract({})));
    onChange([...transforms, { id: newId(), kind: 'filter', mode: 'builder', json, enabled: true }]);
  };

  const update = (id: string, patch: Partial<TransformModel>) =>
    onChange(transforms.map((t) => (t.id === id ? { ...t, ...patch } : t)));

  const remove = (id: string) => onChange(transforms.filter((t) => t.id !== id));

  const changeKind = (t: TransformModel, kind: TransformKind) => {
    const spec = getTransformSpec(kind);
    const json = serializeTransform(spec.build(spec.extract(parseTransformObject(t.json))));
    update(t.id, { kind, json });
  };

  const toggleMode = (t: TransformModel) => {
    const goingRaw = (t.mode ?? 'builder') !== 'raw';
    if (goingRaw && isBlankJson(t.json)) {
      update(t.id, { mode: 'raw', json: TRANSFORM_TEMPLATES[t.kind] });
    } else {
      update(t.id, { mode: goingRaw ? 'raw' : 'builder' });
    }
  };

  return (
    <div className={styles.list}>
      {transforms.map((t) => {
        const raw = (t.mode ?? 'builder') === 'raw';
        return (
          <div key={t.id} className={styles.row}>
            <div className={styles.header}>
              <InlineField label="Kind" labelWidth={8}>
                <Select<TransformKind>
                  width={20}
                  options={TRANSFORM_OPTIONS}
                  value={t.kind}
                  onChange={(v) => v.value && changeKind(t, v.value)}
                />
              </InlineField>
              <div className={styles.spacer} />
              <Button variant="secondary" size="sm" fill="text" onClick={() => toggleMode(t)}>
                {raw ? 'Builder' : 'JSON'}
              </Button>
              <Switch value={t.enabled !== false} onChange={(e) => update(t.id, { enabled: e.currentTarget.checked })} />
              <IconButton name="trash-alt" aria-label="Remove transform" onClick={() => remove(t.id)} />
            </div>
            {raw ? (
              <JsonInput value={t.json} rows={5} onChange={(json) => update(t.id, { json })} />
            ) : (
              <div className={styles.fields}>
                <StructuredFields
                  kind={t.kind}
                  json={t.json}
                  fieldOptions={fieldOptions}
                  onJsonChange={(json) => update(t.id, { json })}
                />
              </div>
            )}
          </div>
        );
      })}
      <Button variant="secondary" size="sm" icon="plus" onClick={add}>
        Add transform
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
    padding: theme.spacing(1),
  }),
  header: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(0.5), marginBottom: theme.spacing(0.5) }),
  spacer: css({ flex: 1 }),
  fields: css({ display: 'flex', flexDirection: 'column' }),
});

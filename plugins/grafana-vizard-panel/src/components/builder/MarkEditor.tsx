import React, { useEffect, useRef, useState } from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2 } from '@grafana/data';
import { Field, Input, Select, Slider, Switch, useStyles2 } from '@grafana/ui';

import { MarkModel, MarkType, PropMap } from '../../types';
import { FillInput } from './FillInput';
import { JsonInput } from './JsonInput';
import { MARK_OPTIONS } from './options';
import { MarkPropDef, MARK_SECTIONS } from './markProperties';

interface Props {
  value: MarkModel;
  onChange: (mark: MarkModel) => void;
}

function parseDash(raw: string): number[] {
  return raw
    .split(',')
    .map((s) => parseFloat(s.trim()))
    .filter((n) => !Number.isNaN(n));
}

/**
 * Stroke-dash input. A plain controlled input would parse "6," to `[6]` and
 * immediately re-render the value back to "6", eating the comma. So we keep the
 * raw typed text in local state (commas and trailing commas survive) and commit
 * the parsed `number[]`, re-syncing only when the external value changes
 * (e.g. a preset / "None" reset).
 */
function DashInput({
  value,
  placeholder,
  onChange,
}: {
  value: unknown;
  placeholder?: string;
  onChange: (v: number[] | undefined) => void;
}) {
  const asText = (v: unknown) => (Array.isArray(v) ? v.join(',') : '');
  const [text, setText] = useState(() => asText(value));
  const lastEmitted = useRef(asText(value));

  useEffect(() => {
    const incoming = asText(value);
    if (incoming !== lastEmitted.current) {
      setText(incoming);
      lastEmitted.current = incoming;
    }
  }, [value]);

  const onText = (raw: string) => {
    setText(raw);
    const arr = parseDash(raw);
    lastEmitted.current = arr.join(',');
    onChange(arr.length > 0 ? arr : undefined);
  };

  return <Input value={text} placeholder={placeholder ?? '6,4'} onChange={(e) => onText(e.currentTarget.value)} />;
}

export function MarkEditor({ value, onChange }: Props) {
  const styles = useStyles2(getStyles);
  const set = (patch: Partial<MarkModel>) => onChange({ ...value, ...patch });

  const setProp = (key: string, v: unknown) => {
    const props: PropMap = { ...(value.props ?? {}) };
    if (v === undefined || v === '') {
      delete props[key];
    } else {
      props[key] = v;
    }
    onChange({ ...value, props: Object.keys(props).length > 0 ? props : undefined });
  };

  const fieldVal = (key: string): unknown => (value as unknown as Record<string, unknown>)[key];
  const propVal = (key: string): unknown => value.props?.[key];
  /** A property reads from the typed field or the props bag depending on its target. */
  const readVal = (def: MarkPropDef): unknown => (def.target === 'field' ? fieldVal(def.key) : propVal(def.key));
  const commit = (def: MarkPropDef, v: unknown) =>
    def.target === 'field'
      ? set({ [def.key as keyof MarkModel]: v } as Partial<MarkModel>)
      : setProp(def.key, v);

  const commitNumber = (def: MarkPropDef, raw: string) => {
    const n = parseFloat(raw);
    commit(def, Number.isNaN(n) ? undefined : n);
  };

  const control = (def: MarkPropDef): React.ReactElement => {
    const v = readVal(def);
    switch (def.kind) {
      case 'number':
        return (
          <Input
            type="number"
            value={typeof v === 'number' ? v : ''}
            placeholder={def.placeholder ?? 'auto'}
            min={def.min}
            max={def.max}
            step={def.step}
            onChange={(e) => commitNumber(def, e.currentTarget.value)}
          />
        );
      case 'slider':
        return (
          <div className={styles.slider}>
            <Slider
              min={def.min ?? 0}
              max={def.max ?? 1}
              step={def.step ?? 0.05}
              value={typeof v === 'number' ? v : def.default ?? 0}
              onAfterChange={(n) => commit(def, typeof n === 'number' ? n : undefined)}
            />
          </div>
        );
      case 'text':
        return (
          <Input
            value={typeof v === 'string' ? v : ''}
            placeholder={def.placeholder}
            onChange={(e) => commit(def, e.currentTarget.value || undefined)}
          />
        );
      case 'fill':
        return <FillInput value={v} onChange={(c) => setProp(def.key, c)} />;
      case 'select':
        return (
          <Select
            options={def.options ?? []}
            value={typeof v === 'string' ? v : ''}
            onChange={(o) => commit(def, o.value || undefined)}
          />
        );
      case 'switch':
        return <Switch value={Boolean(v)} onChange={(e) => commit(def, e.currentTarget.checked)} />;
      case 'dash':
        return <DashInput value={v} placeholder={def.placeholder} onChange={(arr) => setProp(def.key, arr)} />;
    }
  };

  const t = value.type;
  const applies = (a?: MarkType[]) => !a || a.includes(t);

  return (
    <>
      <Field label="Type">
        <Select<MarkType> options={MARK_OPTIONS} value={t} onChange={(v) => v.value && set({ type: v.value })} />
      </Field>

      {MARK_SECTIONS.map((section) => {
        if (!applies(section.appliesTo)) {
          return null;
        }
        const props = section.props.filter((p) => applies(p.appliesTo));
        if (props.length === 0) {
          return null;
        }
        return (
          <div key={section.label} className={styles.section}>
            <div className={styles.sectionLabel}>{section.label}</div>
            {props.map((def) => (
              <Field key={def.key} label={def.label} description={def.description}>
                {control(def)}
              </Field>
            ))}
          </div>
        );
      })}

      <div className={styles.advanced}>
        <JsonInput
          label="Advanced mark properties (optional JSON override)"
          description="Any other Vega-Lite mark property (e.g. gradients, corner radius per corner). Merged last."
          value={value.advancedJson}
          rows={3}
          placeholder='{ "cornerRadiusTopLeft": 4, "line": { "interpolate": "monotone" } }'
          onChange={(json) => set({ advancedJson: json })}
        />
      </div>
    </>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  slider: css({ paddingTop: theme.spacing(1) }),
  section: css({
    marginTop: theme.spacing(2),
    paddingTop: theme.spacing(1.5),
    borderTop: `1px solid ${theme.colors.border.weak}`,
  }),
  sectionLabel: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
    marginBottom: theme.spacing(1),
  }),
  advanced: css({
    marginTop: theme.spacing(3),
    paddingTop: theme.spacing(2),
    borderTop: `1px solid ${theme.colors.border.weak}`,
  }),
});

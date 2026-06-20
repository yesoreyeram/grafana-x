import React from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2, SelectableValue } from '@grafana/data';
import { Button, IconButton, InlineField, InlineFieldRow, Input, Select, useStyles2 } from '@grafana/ui';

import { ParamModel, PropMap } from '../../types';

interface Props {
  params: ParamModel[];
  onChange: (params: ParamModel[]) => void;
}

let idCounter = 0;
function newId(): string {
  idCounter += 1;
  return `param-${Date.now()}-${idCounter}`;
}

const TYPE_OPTIONS: Array<SelectableValue<string>> = [
  { label: 'Interval (brush)', value: 'interval' },
  { label: 'Point (click)', value: 'point' },
  { label: 'Variable / input', value: 'variable' },
];

function paramType(p: ParamModel): string {
  const select = p.spec.select;
  if (select && typeof select === 'object') {
    const t = (select as PropMap).type;
    return typeof t === 'string' ? t : 'point';
  }
  return 'variable';
}

export function ParamEditor({ params, onChange }: Props) {
  const styles = useStyles2(getStyles);

  const add = () =>
    onChange([...params, { id: newId(), name: `param${params.length + 1}`, spec: { select: { type: 'interval' } } }]);
  const update = (id: string, patch: Partial<ParamModel>) =>
    onChange(params.map((p) => (p.id === id ? { ...p, ...patch } : p)));
  const remove = (id: string) => onChange(params.filter((p) => p.id !== id));

  const setType = (p: ParamModel, type: string) => {
    if (type === 'variable') {
      const next: PropMap = { ...p.spec };
      delete next.select;
      update(p.id, { spec: next });
      return;
    }
    const existing = p.spec.select && typeof p.spec.select === 'object' ? (p.spec.select as PropMap) : {};
    update(p.id, { spec: { ...p.spec, select: { ...existing, type } } });
  };

  return (
    <div className={styles.list}>
      {params.length > 0 && (
        <div className={styles.hint}>
          Parameters power interactions (brush/click selections) and input bindings. Reference them in filters or
          conditions.
        </div>
      )}
      {params.map((p) => (
        <div key={p.id} className={styles.row}>
          <InlineFieldRow>
            <InlineField label="Name" labelWidth={8}>
              <Input width={18} value={p.name} onChange={(e) => update(p.id, { name: e.currentTarget.value })} />
            </InlineField>
            <InlineField label="Type">
              <Select width={20} options={TYPE_OPTIONS} value={paramType(p)} onChange={(v) => v.value && setType(p, v.value)} />
            </InlineField>
            <div className={styles.spacer} />
            <IconButton name="trash-alt" aria-label="Remove parameter" onClick={() => remove(p.id)} />
          </InlineFieldRow>
        </div>
      ))}
      <Button variant="secondary" size="sm" icon="plus" onClick={add}>
        Add parameter
      </Button>
    </div>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  list: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  hint: css({ color: theme.colors.text.secondary, fontSize: theme.typography.bodySmall.fontSize }),
  row: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
    padding: theme.spacing(0.5, 1),
  }),
  spacer: css({ flex: 1 }),
});

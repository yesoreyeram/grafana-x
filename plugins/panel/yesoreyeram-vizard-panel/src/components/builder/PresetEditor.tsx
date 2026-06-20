import React, { useState } from 'react';
import { SelectableValue, StandardEditorProps } from '@grafana/data';
import { Select } from '@grafana/ui';

import { buildDataContext } from '../../data/dataContext';
import { applyPreset, PRESETS, PresetId } from '../../spec/presets';
import { BuilderModel, defaultBuilder, PanelOptions } from '../../types';

type Props = StandardEditorProps<BuilderModel, unknown, PanelOptions>;

/** "None" (blank) first, then every preset ordered by group with a group-prefixed description. */
const PRESET_OPTIONS: Array<SelectableValue<PresetId | 'none'>> = [
  { label: 'None (start blank)', value: 'none', description: 'Clear the chart and configure the mark and encodings yourself.' },
  ...PRESETS.map((p) => ({ label: p.label, value: p.id, description: `${p.group} — ${p.description}` })),
];

/**
 * A dropdown of chart-type presets. Choosing one maps the current data onto a
 * complete mark + encodings (folding wide measures or using an existing series
 * dimension as needed) that stays fully editable in the sections below. The
 * Advanced escape-hatch JSON is preserved across changes.
 */
export function PresetEditor({ value, onChange, context }: Props) {
  const [selected, setSelected] = useState<PresetId | 'none' | undefined>(undefined);

  const apply = (id: PresetId | 'none') => {
    const ctx = buildDataContext(context.data ?? [], context.options?.data ?? { source: 'auto' });
    onChange(applyPreset(id, ctx, value ?? { ...defaultBuilder }));
    setSelected(id);
  };

  return (
    <Select
      options={PRESET_OPTIONS}
      value={selected ?? null}
      placeholder="Choose a chart type…"
      aria-label="Chart type preset"
      onChange={(v) => v.value && apply(v.value)}
    />
  );
}

import React from 'react';
import { css } from '@emotion/css';
import { DataFrame, GrafanaTheme2, SelectableValue, StandardEditorProps } from '@grafana/data';
import { useStyles2 } from '@grafana/ui';

import { buildDataContext } from '../../data/dataContext';
import {
  defaultMark,
  EncodingModel,
  LayerModel,
  MarkModel,
  PanelOptions,
  ParamModel,
  TransformModel,
  VegaLiteFieldType,
} from '../../types';
import { EncodingEditor } from './EncodingEditor';
import { JsonInput } from './JsonInput';
import { LayerEditor } from './LayerEditor';
import { MarkEditor } from './MarkEditor';
import { ParamEditor } from './ParamEditor';
import { TransformEditor } from './TransformEditor';

interface Fields {
  fieldOptions: Array<SelectableValue<string>>;
  fieldTypes: Record<string, VegaLiteFieldType>;
}

function getFields(data: DataFrame[] | undefined): Fields {
  const ctx = buildDataContext(data ?? [], { source: 'auto' });
  const fieldOptions = ctx.fields.map((f) => ({ label: f.name, value: f.name, description: f.vegaLiteType }));
  const fieldTypes: Record<string, VegaLiteFieldType> = {};
  ctx.fields.forEach((f) => {
    fieldTypes[f.name] = f.vegaLiteType;
  });
  return { fieldOptions, fieldTypes };
}

function isLayered(context: { options?: PanelOptions }): boolean {
  return (context.options?.builder?.layers?.length ?? 0) > 0;
}

function Hint({ children }: { children: React.ReactNode }) {
  const styles = useStyles2(getStyles);
  return <div className={styles.hint}>{children}</div>;
}

export function MarkSectionEditor({ value, onChange, context }: StandardEditorProps<MarkModel, unknown, PanelOptions>) {
  if (isLayered(context)) {
    return <Hint>This single mark is ignored while layers are defined. Edit each layer&apos;s mark in the Layers section.</Hint>;
  }
  return <MarkEditor value={value ?? { ...defaultMark }} onChange={onChange} />;
}

export function EncodingSectionEditor({
  value,
  onChange,
  context,
}: StandardEditorProps<EncodingModel[], unknown, PanelOptions>) {
  const { fieldOptions, fieldTypes } = getFields(context.data);
  return (
    <>
      {isLayered(context) && <Hint>Shared across all layers. Per-layer encodings live in the Layers section.</Hint>}
      <EncodingEditor encodings={value ?? []} fieldOptions={fieldOptions} fieldTypes={fieldTypes} onChange={onChange} />
    </>
  );
}

export function LayersSectionEditor({ value, onChange, context }: StandardEditorProps<LayerModel[], unknown, PanelOptions>) {
  const { fieldOptions, fieldTypes } = getFields(context.data);
  return (
    <>
      <Hint>Add layers to draw multiple marks on shared axes (e.g. line + points, bar + rule).</Hint>
      <LayerEditor
        layers={value ?? []}
        fieldOptions={fieldOptions}
        fieldTypes={fieldTypes}
        defaultLayerMark={context.options?.builder?.mark}
        onChange={onChange}
      />
    </>
  );
}

export function TransformsSectionEditor({
  value,
  onChange,
  context,
}: StandardEditorProps<TransformModel[], unknown, PanelOptions>) {
  const { fieldOptions } = getFields(context.data);
  return <TransformEditor transforms={value ?? []} fieldOptions={fieldOptions} onChange={onChange} />;
}

export function ParamsSectionEditor({ value, onChange }: StandardEditorProps<ParamModel[], unknown, PanelOptions>) {
  return <ParamEditor params={value ?? []} onChange={onChange} />;
}

export function ConfigJsonEditor({ value, onChange }: StandardEditorProps<string, unknown, PanelOptions>) {
  return (
    <JsonInput
      label="Vega-Lite config (theme overrides)"
      description="Merged on top of the Grafana theme config. Most users won't need this."
      value={value}
      rows={5}
      placeholder='{ "axis": { "grid": false } }'
      onChange={onChange}
    />
  );
}

export function SpecOverrideEditor({ value, onChange }: StandardEditorProps<string, unknown, PanelOptions>) {
  return (
    <JsonInput
      label="Spec override (deep-merged last)"
      description="Any Vega-Lite property — the escape hatch for multi-view (facet/concat/repeat) and geo."
      value={value}
      rows={8}
      placeholder='{ "encoding": { "y": { "scale": { "zero": false } } } }'
      onChange={onChange}
    />
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  hint: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    marginBottom: theme.spacing(1),
  }),
});

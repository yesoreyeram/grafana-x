import React, { useState } from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2, SelectableValue } from '@grafana/data';
import { Button, IconButton, useStyles2 } from '@grafana/ui';

import { defaultMark, LayerModel, MarkModel, VegaLiteFieldType } from '../../types';
import { EncodingEditor } from './EncodingEditor';
import { MarkEditor } from './MarkEditor';

interface Props {
  layers: LayerModel[];
  fieldOptions: Array<SelectableValue<string>>;
  fieldTypes: Record<string, VegaLiteFieldType>;
  /** The current single-view mark; seeds the FIRST layer so it isn't lost. */
  defaultLayerMark?: MarkModel;
  onChange: (layers: LayerModel[]) => void;
}

let idCounter = 0;
function newId(): string {
  idCounter += 1;
  return `layer-${Date.now()}-${idCounter}`;
}

export function LayerEditor({ layers, fieldOptions, fieldTypes, defaultLayerMark, onChange }: Props) {
  const styles = useStyles2(getStyles);
  const [openIds, setOpenIds] = useState<Set<string>>(new Set());

  const toggle = (id: string) =>
    setOpenIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });

  const add = () => {
    const id = newId();
    // Seed the first layer from the current single-view mark so it migrates
    // cleanly (the top-level mark is otherwise ignored once layers exist).
    const seedMark = layers.length === 0 && defaultLayerMark ? { ...defaultLayerMark } : { ...defaultMark };
    onChange([...layers, { id, mark: seedMark, encodings: [] }]);
    setOpenIds((prev) => new Set(prev).add(id));
  };
  const update = (id: string, patch: Partial<LayerModel>) =>
    onChange(layers.map((l) => (l.id === id ? { ...l, ...patch } : l)));
  const remove = (id: string) => onChange(layers.filter((l) => l.id !== id));

  return (
    <div className={styles.list}>
      {layers.length > 0 && (
        <div className={styles.hint}>Each layer is drawn on top of the previous one. Shared encodings go in the Encoding section above.</div>
      )}
      {layers.map((layer, i) => (
        <div key={layer.id} className={styles.row}>
          <div className={styles.header}>
            <IconButton name={openIds.has(layer.id) ? 'angle-down' : 'angle-right'} aria-label="Expand" onClick={() => toggle(layer.id)} />
            <button type="button" className={styles.summary} onClick={() => toggle(layer.id)}>
              {`Layer ${i + 1}: ${layer.mark.type}`}
            </button>
            <IconButton name="trash-alt" aria-label="Remove layer" onClick={() => remove(layer.id)} />
          </div>
          {openIds.has(layer.id) && (
            <div className={styles.body}>
              <MarkEditor value={layer.mark} onChange={(mark) => update(layer.id, { mark })} />
              <EncodingEditor
                encodings={layer.encodings}
                fieldOptions={fieldOptions}
                fieldTypes={fieldTypes}
                onChange={(encodings) => update(layer.id, { encodings })}
              />
            </div>
          )}
        </div>
      ))}
      <Button variant="secondary" size="sm" icon="plus" onClick={add}>
        Add layer
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
  }),
  header: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(0.5), padding: theme.spacing(0.5, 1) }),
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
  body: css({ padding: theme.spacing(1), borderTop: `1px solid ${theme.colors.border.weak}` }),
});

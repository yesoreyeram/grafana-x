import React from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2, SelectableValue } from '@grafana/data';
import { ColorPickerInput, RadioButtonGroup, Select, useStyles2 } from '@grafana/ui';

/**
 * A color input that also produces Vega-Lite gradient fills. The value it
 * emits is either a solid color string or a Vega-Lite
 * [gradient](https://vega.github.io/vega-lite/docs/gradient.html) object
 * (`{ gradient: 'linear' | 'radial', stops: [...] }`), stored verbatim in the
 * mark's `props` bag and merged onto the mark by `fromBuilder`. Two stops
 * (start / end) are exposed here; multi-stop gradients remain available via the
 * Advanced mark JSON.
 */
type Mode = 'solid' | 'linear' | 'radial';

interface GradientStop {
  offset: number;
  color: string;
}
interface Gradient {
  gradient: 'linear' | 'radial';
  x1?: number;
  y1?: number;
  x2?: number;
  y2?: number;
  stops: GradientStop[];
}

export type FillValue = string | Gradient | undefined;

const MODE_OPTIONS: Array<SelectableValue<Mode>> = [
  { label: 'Solid', value: 'solid' },
  { label: 'Linear', value: 'linear' },
  { label: 'Radial', value: 'radial' },
];

interface Direction {
  label: string;
  value: string;
  coords: { x1: number; y1: number; x2: number; y2: number };
}
const DIRECTIONS: Direction[] = [
  { label: 'Top → bottom', value: 'tb', coords: { x1: 0, y1: 0, x2: 0, y2: 1 } },
  { label: 'Bottom → top', value: 'bt', coords: { x1: 0, y1: 1, x2: 0, y2: 0 } },
  { label: 'Left → right', value: 'lr', coords: { x1: 0, y1: 0, x2: 1, y2: 0 } },
  { label: 'Right → left', value: 'rl', coords: { x1: 1, y1: 0, x2: 0, y2: 0 } },
  { label: 'Diagonal', value: 'diag', coords: { x1: 0, y1: 0, x2: 1, y2: 1 } },
];

const DEFAULT_START = '#73BF69';
const DEFAULT_END = '#1F78C1';

function isGradient(value: unknown): value is Gradient {
  return typeof value === 'object' && value !== null && 'gradient' in value;
}

interface Props {
  value: unknown;
  onChange: (value: FillValue) => void;
}

export function FillInput({ value, onChange }: Props) {
  const styles = useStyles2(getStyles);

  const grad = isGradient(value) ? value : undefined;
  const mode: Mode = grad ? (grad.gradient === 'radial' ? 'radial' : 'linear') : 'solid';
  const solid = typeof value === 'string' ? value : '';
  const start = grad?.stops?.[0]?.color ?? (solid || DEFAULT_START);
  const end = grad?.stops?.[grad.stops.length - 1]?.color ?? DEFAULT_END;
  const dirValue =
    (grad &&
      DIRECTIONS.find((d) => d.coords.x1 === grad.x1 && d.coords.y1 === grad.y1 && d.coords.x2 === grad.x2 && d.coords.y2 === grad.y2)
        ?.value) ||
    'tb';

  const emit = (m: Mode, s: string, e: string, dir: string) => {
    if (m === 'solid') {
      onChange(s || undefined);
      return;
    }
    const stops: GradientStop[] = [
      { offset: 0, color: s || DEFAULT_START },
      { offset: 1, color: e || DEFAULT_END },
    ];
    if (m === 'radial') {
      onChange({ gradient: 'radial', stops });
      return;
    }
    const coords = (DIRECTIONS.find((d) => d.value === dir) ?? DIRECTIONS[0]).coords;
    onChange({ gradient: 'linear', ...coords, stops });
  };

  return (
    <div className={styles.wrap}>
      <RadioButtonGroup size="sm" fullWidth options={MODE_OPTIONS} value={mode} onChange={(m) => m && emit(m, start, end, dirValue)} />

      {mode === 'solid' ? (
        <ColorPickerInput value={solid} returnColorAs="hex" onChange={(c) => onChange(c || undefined)} />
      ) : (
        <>
          <div className={styles.colors}>
            <div className={styles.col}>
              <ColorPickerInput value={start} returnColorAs="hex" onChange={(c) => emit(mode, c, end, dirValue)} />
            </div>
            <div className={styles.col}>
              <ColorPickerInput value={end} returnColorAs="hex" onChange={(c) => emit(mode, start, c, dirValue)} />
            </div>
          </div>
          {mode === 'linear' && (
            <Select options={DIRECTIONS} value={dirValue} onChange={(o) => o.value && emit('linear', start, end, o.value)} />
          )}
        </>
      )}
    </div>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  colors: css({ display: 'flex', gap: theme.spacing(1) }),
  col: css({ flex: 1, minWidth: 0 }),
});

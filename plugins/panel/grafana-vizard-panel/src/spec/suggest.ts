import { DataContext, FieldInfo } from '../data/dataContext';
import { SpecObject } from '../types';
import { escapeFieldName } from './field';

function channel(name: string, type: string, extra?: SpecObject): SpecObject {
  return { field: escapeFieldName(name), type, ...(extra ?? {}) };
}

function countY(): SpecObject {
  return { aggregate: 'count', type: 'quantitative', title: 'Count' };
}

function nonColliding(base: string, used: Set<string>): string {
  if (!used.has(base)) {
    return base;
  }
  let i = 2;
  while (used.has(`${base}${i}`)) {
    i++;
  }
  return `${base}${i}`;
}

function lineMark(): SpecObject {
  return { type: 'line', tooltip: true, point: true };
}

function barMark(): SpecObject {
  return { type: 'bar', tooltip: true };
}

function pointMark(): SpecObject {
  return { type: 'point', tooltip: true, filled: true };
}

/**
 * Produce a sensible starting spec from the detected data shape. Used when the
 * builder has no encodings configured yet, so an empty panel still renders a
 * meaningful chart that the user can then refine.
 */
export function suggestSpec(ctx: DataContext): SpecObject {
  const fields = ctx.fields;
  if (fields.length === 0) {
    return { mark: pointMark(), encoding: {} };
  }

  const used = new Set(fields.map((f) => f.name));
  const temporal = fields.filter((f) => f.vegaLiteType === 'temporal');
  const quant = fields.filter((f) => f.vegaLiteType === 'quantitative');
  const nominal = fields.filter((f) => f.vegaLiteType !== 'temporal' && f.vegaLiteType !== 'quantitative');
  const index = ctx.indexField ?? fields[0].name;
  const firstName = (list: FieldInfo[]): string | undefined => list[0]?.name;

  // Logs: counts over time (coloured by level/severity when present).
  if (ctx.kind === 'logs') {
    const time = firstName(temporal);
    const level = nominal.find((f) => /level|severity|status/i.test(f.name));
    const encoding: SpecObject = {
      x: time ? channel(time, 'temporal') : channel(firstName(nominal) ?? fields[0].name, 'nominal'),
      y: countY(),
    };
    if (level) {
      encoding.color = channel(level.name, 'nominal');
    }
    return { mark: barMark(), encoding };
  }

  // Long / merged series: one value column plus a series/label dimension.
  if (ctx.seriesField) {
    const y = firstName(quant);
    const encoding: SpecObject = {
      x: channel(index, temporal.length ? 'temporal' : 'nominal'),
      y: y ? channel(y, 'quantitative') : countY(),
      color: channel(ctx.seriesField, 'nominal'),
    };
    return { mark: temporal.length ? lineMark() : barMark(), encoding };
  }

  // Time series: temporal index + one or more numeric values.
  if (temporal.length > 0 && quant.length > 0) {
    if (quant.length > 1) {
      const key = nonColliding('key', used);
      const value = nonColliding('value', used);
      return {
        transform: [{ fold: quant.map((q) => escapeFieldName(q.name)), as: [key, value] }],
        mark: lineMark(),
        encoding: {
          x: channel(index, 'temporal'),
          y: channel(value, 'quantitative'),
          color: channel(key, 'nominal'),
        },
      };
    }
    return {
      mark: lineMark(),
      encoding: {
        x: channel(index, 'temporal'),
        y: channel(quant[0].name, 'quantitative'),
      },
    };
  }

  // Numeric, no time.
  if (quant.length > 0) {
    if (quant.length > 1 && nominal.length === 0) {
      const key = nonColliding('metric', used);
      const value = nonColliding('value', used);
      return {
        transform: [{ fold: quant.map((q) => escapeFieldName(q.name)), as: [key, value] }],
        mark: barMark(),
        encoding: {
          x: channel(key, 'nominal'),
          y: channel(value, 'quantitative'),
          color: channel(key, 'nominal'),
        },
      };
    }
    const encoding: SpecObject = { y: channel(quant[0].name, 'quantitative') };
    if (nominal.length > 0) {
      encoding.x = channel(nominal[0].name, 'nominal');
    }
    if (nominal.length > 1) {
      encoding.color = channel(nominal[1].name, 'nominal');
    }
    return { mark: barMark(), encoding };
  }

  // No numeric fields: count by the first discrete field.
  if (nominal.length > 0) {
    const encoding: SpecObject = {
      x: channel(nominal[0].name, 'nominal'),
      y: countY(),
    };
    if (nominal.length > 1) {
      encoding.color = channel(nominal[1].name, 'nominal');
    }
    return { mark: barMark(), encoding };
  }

  // Absolute fallback: scatter the first two columns.
  const encoding: SpecObject = { x: channel(fields[0].name, fields[0].vegaLiteType) };
  if (fields[1]) {
    encoding.y = channel(fields[1].name, fields[1].vegaLiteType);
  }
  return { mark: pointMark(), encoding };
}

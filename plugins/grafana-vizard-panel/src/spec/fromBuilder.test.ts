import { BuilderModel } from '../types';
import { fromBuilder } from './fromBuilder';

function model(partial: Partial<BuilderModel>): BuilderModel {
  return { mark: { type: 'bar' }, encodings: [], transforms: [], ...partial };
}

describe('fromBuilder', () => {
  it('builds mark, field/value encodings, and array channels', () => {
    const { spec, errors } = fromBuilder(
      model({
        mark: { type: 'bar', tooltip: true },
        encodings: [
          { id: '1', channel: 'x', field: 'cat', type: 'nominal' },
          { id: '2', channel: 'y', field: 'val', type: 'quantitative', aggregate: 'sum' },
          { id: '3', channel: 'color', value: 'red' },
          { id: '4', channel: 'tooltip', field: 'a' },
          { id: '5', channel: 'tooltip', field: 'b' },
          { id: '6', channel: 'y', field: 'ignored', enabled: false },
        ],
      })
    );

    expect(errors).toHaveLength(0);
    expect(spec.mark).toEqual({ type: 'bar', tooltip: true });

    const enc = spec.encoding as Record<string, unknown>;
    expect(enc.x).toEqual({ field: 'cat', type: 'nominal' });
    expect(enc.y).toEqual({ field: 'val', type: 'quantitative', aggregate: 'sum' });
    expect(enc.color).toEqual({ value: 'red' });
    expect(enc.tooltip).toEqual([{ field: 'a' }, { field: 'b' }]);
  });

  it('supports count aggregate without a field and stack none -> null', () => {
    const { spec } = fromBuilder(
      model({
        encodings: [
          { id: '1', channel: 'x', field: 't', type: 'temporal' },
          { id: '2', channel: 'y', aggregate: 'count' },
          { id: '3', channel: 'y', field: 'v', type: 'quantitative', stack: 'none' },
        ],
      })
    );
    const enc = spec.encoding as Record<string, unknown>;
    // last y wins (stack none -> null)
    expect(enc.y).toEqual({ field: 'v', type: 'quantitative', stack: null });
  });

  it('collapses a bare mark to the string shorthand', () => {
    const { spec } = fromBuilder(model({ mark: { type: 'line' }, encodings: [{ id: '1', channel: 'x', field: 'a' }] }));
    expect(spec.mark).toBe('line');
  });

  it('parses transforms and reports invalid JSON', () => {
    const { spec, errors } = fromBuilder(
      model({
        encodings: [{ id: '1', channel: 'x', field: 'a' }],
        transforms: [
          { id: 't1', kind: 'filter', json: '{"filter":"datum.a>0"}' },
          { id: 't2', kind: 'calculate', json: '{ not json' },
          { id: 't3', kind: 'filter', json: '{"filter":"x"}', enabled: false },
        ],
      })
    );
    expect(spec.transform).toEqual([{ filter: 'datum.a>0' }]);
    expect(errors.some((e) => e.includes('calculate'))).toBe(true);
  });

  it('deep-merges per-channel advanced JSON', () => {
    const { spec } = fromBuilder(
      model({
        encodings: [{ id: '1', channel: 'color', field: 'c', type: 'nominal', advancedJson: '{"scale":{"scheme":"blues"}}' }],
      })
    );
    const enc = spec.encoding as Record<string, unknown>;
    expect(enc.color).toEqual({ field: 'c', type: 'nominal', scale: { scheme: 'blues' } });
  });

  it('merges structured mark props (typed builder controls)', () => {
    const { spec } = fromBuilder(
      model({
        mark: { type: 'bar', tooltip: true, props: { cornerRadius: 4, size: 20, strokeDash: [6, 4] } },
        encodings: [{ id: '1', channel: 'x', field: 'a' }],
      })
    );
    expect(spec.mark).toEqual({ type: 'bar', tooltip: true, cornerRadius: 4, size: 20, strokeDash: [6, 4] });
  });

  it('passes a gradient fill/stroke object through to the mark', () => {
    const gradient = {
      gradient: 'linear',
      x1: 0,
      y1: 0,
      x2: 0,
      y2: 1,
      stops: [
        { offset: 0, color: '#3a1c71' },
        { offset: 1, color: '#ffaf7b' },
      ],
    };
    const { spec } = fromBuilder(
      model({ mark: { type: 'area', tooltip: true, props: { fill: gradient } }, encodings: [{ id: '1', channel: 'x', field: 't' }] })
    );
    expect((spec.mark as Record<string, unknown>).fill).toEqual(gradient);
  });

  it('drops null mark props left by a reset', () => {
    const { spec } = fromBuilder(
      model({ mark: { type: 'bar', tooltip: true, props: { cornerRadius: 4, strokeWidth: null } } })
    );
    expect(spec.mark).toEqual({ type: 'bar', tooltip: true, cornerRadius: 4 });
  });

  it('passes through the full mark-property set (color/stroke/text/arc props)', () => {
    const { spec } = fromBuilder(
      model({
        mark: {
          type: 'arc',
          filled: true,
          opacity: 0.8,
          props: {
            fill: '#ff8800',
            stroke: '#000000',
            fillOpacity: 0.5,
            strokeCap: 'round',
            strokeJoin: 'bevel',
            blend: 'multiply',
            cursor: 'pointer',
            invalid: 'filter',
            innerRadius: 60,
            padAngle: 0.02,
            align: 'center',
          },
        },
        encodings: [{ id: '1', channel: 'theta', field: 'a', type: 'quantitative' }],
      })
    );
    expect(spec.mark).toEqual({
      type: 'arc',
      filled: true,
      opacity: 0.8,
      fill: '#ff8800',
      stroke: '#000000',
      fillOpacity: 0.5,
      strokeCap: 'round',
      strokeJoin: 'bevel',
      blend: 'multiply',
      cursor: 'pointer',
      invalid: 'filter',
      innerRadius: 60,
      padAngle: 0.02,
      align: 'center',
    });
  });
});

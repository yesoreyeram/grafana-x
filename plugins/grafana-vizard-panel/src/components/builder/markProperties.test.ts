import { MarkModel } from '../../types';
import { MARK_SECTIONS } from './markProperties';

// The five typed MarkModel fields a property may target with `target: 'field'`.
const TYPED_FIELDS: Array<keyof MarkModel> = ['opacity', 'tooltip', 'point', 'filled', 'interpolate'];

describe('mark property schema', () => {
  const allProps = MARK_SECTIONS.flatMap((s) => s.props);

  it('gives every property a key, label and kind', () => {
    for (const p of allProps) {
      expect(p.key).toBeTruthy();
      expect(p.label).toBeTruthy();
      expect(p.kind).toBeTruthy();
    }
  });

  it('uses unique property keys within each section', () => {
    for (const section of MARK_SECTIONS) {
      const keys = section.props.map((p) => p.key);
      expect(new Set(keys).size).toBe(keys.length);
    }
  });

  it('only targets known typed fields with target: "field"', () => {
    for (const p of allProps.filter((p) => p.target === 'field')) {
      expect(TYPED_FIELDS).toContain(p.key as keyof MarkModel);
    }
  });

  it('provides options for every select and min/max for every slider', () => {
    for (const p of allProps) {
      if (p.kind === 'select') {
        expect(Array.isArray(p.options) && p.options!.length > 0).toBe(true);
      }
      if (p.kind === 'slider') {
        expect(typeof p.min).toBe('number');
        expect(typeof p.max).toBe('number');
      }
    }
  });

  it('every select option list has a clearable "(default)" empty entry', () => {
    for (const p of allProps.filter((p) => p.kind === 'select')) {
      expect(p.options!.some((o) => o.value === '')).toBe(true);
    }
  });

  it('covers the standard mark-def property groups', () => {
    const labels = MARK_SECTIONS.map((s) => s.label);
    expect(labels).toEqual(
      expect.arrayContaining(['General', 'Color', 'Stroke', 'Line & area', 'Bar', 'Point & symbol', 'Arc', 'Text'])
    );
    // A representative property from each standard group is present.
    const keys = new Set(allProps.map((p) => p.key));
    ['clip', 'cursor', 'blend', 'fill', 'stroke', 'fillOpacity', 'strokeCap', 'strokeJoin', 'shape', 'innerRadius', 'align', 'fontSize'].forEach(
      (k) => expect(keys.has(k)).toBe(true)
    );
  });
});

import { FieldType } from '@grafana/data';

import { vegaLiteTypeForField } from './fieldType';

describe('vegaLiteTypeForField', () => {
  it('maps time to temporal', () => {
    expect(vegaLiteTypeForField({ type: FieldType.time })).toBe('temporal');
  });

  it('maps number to quantitative', () => {
    expect(vegaLiteTypeForField({ type: FieldType.number })).toBe('quantitative');
  });

  it('maps string/boolean/enum to nominal', () => {
    expect(vegaLiteTypeForField({ type: FieldType.string })).toBe('nominal');
    expect(vegaLiteTypeForField({ type: FieldType.boolean })).toBe('nominal');
    expect(vegaLiteTypeForField({ type: FieldType.enum })).toBe('nominal');
  });

  it('falls back to nominal for other types', () => {
    expect(vegaLiteTypeForField({ type: FieldType.other })).toBe('nominal');
  });
});

import { Field, FieldType } from '@grafana/data';

import { VegaLiteFieldType } from '../types';

/**
 * Map a Grafana field type to the closest Vega-Lite measurement type.
 *
 * - time     -> temporal   (Grafana time fields are epoch-ms numbers, which
 *                           Vega-Lite interprets as temporal values)
 * - number   -> quantitative
 * - boolean  -> nominal
 * - enum     -> nominal
 * - string   -> nominal
 * - other    -> nominal    (safe default; nominal works for any discrete value)
 */
export function vegaLiteTypeForField(field: Pick<Field, 'type'>): VegaLiteFieldType {
  switch (field.type) {
    case FieldType.time:
      return 'temporal';
    case FieldType.number:
      return 'quantitative';
    case FieldType.boolean:
    case FieldType.enum:
    case FieldType.string:
      return 'nominal';
    default:
      return 'nominal';
  }
}

import { SpecObject } from '../types';

export function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/**
 * Deep-merge `override` onto `base`. Objects are merged recursively; arrays and
 * scalars from `override` replace those in `base`. Neither input is mutated.
 */
export function deepMerge(base: Record<string, unknown>, override: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = { ...base };
  for (const [key, value] of Object.entries(override)) {
    const existing = out[key];
    if (isPlainObject(existing) && isPlainObject(value)) {
      out[key] = deepMerge(existing, value);
    } else {
      out[key] = value;
    }
  }
  return out;
}

export interface JsonParseResult {
  value?: Record<string, unknown>;
  error?: string;
}

/** Parse a JSON object from text. Empty/blank input is valid and yields no value. */
export function parseJsonObject(text?: string): JsonParseResult {
  if (!text || !text.trim()) {
    return {};
  }
  try {
    const parsed: unknown = JSON.parse(text);
    if (!isPlainObject(parsed)) {
      return { error: 'Expected a JSON object' };
    }
    return { value: parsed };
  } catch (e) {
    return { error: e instanceof Error ? e.message : 'Invalid JSON' };
  }
}

/** Parse any JSON value from text (object, array, or scalar). */
export function parseJsonValue(text?: string): { value?: unknown; error?: string } {
  if (!text || !text.trim()) {
    return {};
  }
  try {
    return { value: JSON.parse(text) as unknown };
  } catch (e) {
    return { error: e instanceof Error ? e.message : 'Invalid JSON' };
  }
}

export function asSpecObject(value: SpecObject): SpecObject {
  return value;
}

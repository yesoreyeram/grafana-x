import React, { useEffect, useState } from 'react';
import { Field, TextArea } from '@grafana/ui';

interface Props {
  label?: string;
  description?: string;
  value?: string;
  placeholder?: string;
  rows?: number;
  onChange: (value: string) => void;
}

function validate(text: string): string | undefined {
  if (!text.trim()) {
    return undefined;
  }
  try {
    JSON.parse(text);
    return undefined;
  } catch (e) {
    return e instanceof Error ? e.message : 'Invalid JSON';
  }
}

/**
 * A JSON escape-hatch text area. Commits on blur (so typing stays smooth) and
 * shows a parse error inline. These inputs give the builder full Vega-Lite
 * grammar coverage for properties that don't have a dedicated control yet.
 */
export function JsonInput({ label, description, value, placeholder, rows = 4, onChange }: Props) {
  const [text, setText] = useState(value ?? '');

  useEffect(() => {
    setText(value ?? '');
  }, [value]);

  const error = validate(text);

  return (
    <Field label={label} description={description} invalid={Boolean(error)} error={error}>
      <TextArea
        value={text}
        rows={rows}
        placeholder={placeholder}
        spellCheck={false}
        onChange={(e) => setText(e.currentTarget.value)}
        onBlur={(e) => onChange(e.currentTarget.value)}
      />
    </Field>
  );
}

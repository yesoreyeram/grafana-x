import React from 'react';
import { Alert } from '@grafana/ui';

interface Props {
  title: string;
  message?: string;
  severity?: 'error' | 'warning' | 'info';
}

/** Small, scrollable, theme-aware error/empty surface used inside the panel. */
export function ErrorView({ title, message, severity = 'error' }: Props) {
  return (
    <div style={{ height: '100%', width: '100%', overflow: 'auto', padding: 8 }}>
      <Alert title={title} severity={severity}>
        {message}
      </Alert>
    </div>
  );
}

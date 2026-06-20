import React, { useState } from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2, StandardEditorProps } from '@grafana/data';
import { Button, ClipboardButton, CodeEditor, Modal, useStyles2 } from '@grafana/ui';

import { fromBuilder } from '../../spec/fromBuilder';
import { PanelOptions } from '../../types';

type Props = StandardEditorProps<unknown, unknown, PanelOptions>;

function generatedJson(options: PanelOptions | undefined): string {
  if (!options?.builder) {
    return '{}';
  }
  try {
    const { spec } = fromBuilder(options.builder);
    return JSON.stringify(spec, null, 2);
  } catch (e) {
    return `// Could not generate spec: ${e instanceof Error ? e.message : String(e)}`;
  }
}

/**
 * Read-only preview of the Vega-Lite grammar the builder generates (mark /
 * encoding / layers / transforms / params). Data, theme config and sizing are
 * injected at render time and are not shown here. A button opens the full JSON
 * in a modal with a copy action.
 */
export function PreviewEditor({ context }: Props) {
  const styles = useStyles2(getStyles);
  const [open, setOpen] = useState(false);
  const json = generatedJson(context.options);

  return (
    <div>
      <pre className={styles.preview} data-testid="vizard-json-preview">
        {json}
      </pre>
      <div className={styles.actions}>
        <Button size="sm" variant="secondary" icon="eye" onClick={() => setOpen(true)}>
          View JSON
        </Button>
        <ClipboardButton size="sm" variant="secondary" icon="copy" getText={() => json}>
          Copy
        </ClipboardButton>
      </div>
      {open && (
        <Modal title="Generated Vega-Lite grammar" isOpen onDismiss={() => setOpen(false)}>
          <div className={styles.modalBody}>
            <CodeEditor
              language="json"
              value={json}
              readOnly
              showMiniMap={false}
              showLineNumbers
              width="100%"
              height="60vh"
              monacoOptions={{ wordWrap: 'on', scrollBeyondLastLine: false }}
            />
          </div>
          <Modal.ButtonRow>
            <ClipboardButton variant="secondary" icon="copy" getText={() => json}>
              Copy
            </ClipboardButton>
            <Button variant="secondary" onClick={() => setOpen(false)}>
              Close
            </Button>
          </Modal.ButtonRow>
        </Modal>
      )}
    </div>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  preview: css({
    maxHeight: 220,
    overflow: 'auto',
    margin: 0,
    padding: theme.spacing(1),
    background: theme.colors.background.canvas,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    color: theme.colors.text.primary,
    whiteSpace: 'pre',
  }),
  actions: css({ display: 'flex', gap: theme.spacing(1), marginTop: theme.spacing(1) }),
  modalBody: css({ width: '100%', overflow: 'hidden' }),
});

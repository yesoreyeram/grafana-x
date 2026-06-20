import React, { useEffect, useRef, useState } from 'react';
import { loader as createVegaLoader } from 'vega';
import embed, { Result, VisualizationSpec } from 'vega-embed';

import { RendererType, VegaLiteSpec } from '../types';
import { extractZoomRange, ZOOM_PARAM } from '../spec/injectZoom';
import { ErrorView } from './ErrorView';

interface Props {
  spec: VegaLiteSpec;
  renderer: RendererType;
  tooltip: boolean;
  /** 'dark' | 'light' — themes the Vega tooltip to match Grafana. */
  tooltipTheme: 'dark' | 'light';
  /** Multi-view (facet/concat/repeat) specs size by content — scroll them instead
   *  of clipping. Single/layered views fit the panel exactly, so they stay clipped
   *  (no spurious scrollbar from the canvas's inline descender gap). */
  scrollable?: boolean;
  /** Called with [from, to] (epoch ms) when the user brushes a temporal x range. */
  onBrush?: (from: number, to: number) => void;
  onError?: (error: Error) => void;
}

/**
 * A Vega loader that rejects every remote/file load. Combined with the spec
 * sanitizer (which strips `url`), this guarantees the chart can never reach the
 * network — no SSRF, no data exfiltration — even for future user-authored specs.
 */
function createBlockingLoader() {
  const loader = createVegaLoader();
  const blocked = () => Promise.reject(new Error('Remote resource loading is disabled in Vizard'));
  loader.load = blocked;
  loader.sanitize = blocked;
  loader.http = blocked;
  loader.file = blocked;
  return loader;
}

/**
 * Renders a Vega-Lite spec with Vega-Embed under hardened, non-overridable
 * options:
 *  - `ast: true`      CSP-safe expression interpreter (no `new Function`)
 *  - blocking loader  no network access
 *  - `actions: false` the Vega export/source/editor menu is disabled so specs
 *                     never leave Grafana
 *
 * It also bridges Vega interactions to Grafana: a temporal x interval selection
 * (added by `injectZoom`) is forwarded to `onBrush` so the panel can update the
 * dashboard time range, like the native time series panel.
 *
 * This module statically imports vega/vega-lite/vega-embed and is loaded lazily
 * by Vizard so the panel's `module.js` stays small.
 */
export default function VegaView({ spec, renderer, tooltip, tooltipTheme, scrollable, onBrush, onError }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | undefined>();
  const onBrushRef = useRef(onBrush);

  useEffect(() => {
    onBrushRef.current = onBrush;
  }, [onBrush]);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) {
      return;
    }

    let cancelled = false;
    let result: Result | undefined;
    let zoomTimer: ReturnType<typeof setTimeout> | undefined;
    setError(undefined);

    embed(el, spec as unknown as VisualizationSpec, {
      mode: 'vega-lite',
      ast: true,
      renderer,
      tooltip: tooltip ? { theme: tooltipTheme } : false,
      actions: false,
      loader: createBlockingLoader(),
    })
      .then((r) => {
        if (cancelled) {
          r.finalize();
          return;
        }
        result = r;
        // Bridge the temporal x interval selection to the dashboard time range.
        try {
          r.view.addSignalListener(ZOOM_PARAM, (_name, value) => {
            const range = extractZoomRange(value);
            if (!range) {
              return;
            }
            if (zoomTimer) {
              clearTimeout(zoomTimer);
            }
            // Debounce so we apply once the user finishes dragging.
            zoomTimer = setTimeout(() => onBrushRef.current?.(range[0], range[1]), 350);
          });
        } catch {
          // No zoom param on this spec — nothing to listen to.
        }
      })
      .catch((e: unknown) => {
        if (cancelled) {
          return;
        }
        const err = e instanceof Error ? e : new Error(String(e));
        setError(err.message);
        onError?.(err);
      });

    return () => {
      cancelled = true;
      if (zoomTimer) {
        clearTimeout(zoomTimer);
      }
      result?.finalize();
    };
  }, [spec, renderer, tooltip, tooltipTheme, onError]);

  if (error) {
    return <ErrorView title="Could not render the Vega-Lite spec" message={error} />;
  }

  // Single/layered views use `autosize: fit` and fill the box exactly, so they
  // stay `hidden` — otherwise the canvas's inline descender gap would trigger a
  // spurious scrollbar. Multi-view specs size by content and can exceed the
  // panel, so they scroll instead of being clipped.
  return (
    <div ref={containerRef} style={{ width: '100%', height: '100%', overflow: scrollable ? 'auto' : 'hidden' }} />
  );
}

import { createTheme } from '@grafana/data';

import { buildVegaConfig } from './grafanaTheme';

describe('buildVegaConfig', () => {
  const theme = createTheme();

  it('always applies Grafana structural theming with a transparent background', () => {
    const cfg = buildVegaConfig(theme, { colorScheme: 'palette-classic' });
    expect(cfg.background).toBe('transparent');
    expect(cfg.font).toBe(theme.typography.fontFamily);
    expect((cfg.view as Record<string, unknown>).stroke).toBeNull();
    expect((cfg.axis as Record<string, unknown>).gridColor).toBe(theme.colors.border.weak);
  });

  it('resolves Grafana color schemes to a theme-aware color array + mark color', () => {
    const cfg = buildVegaConfig(theme, { colorScheme: 'palette-classic' });
    const range = cfg.range as Record<string, unknown>;
    expect(Array.isArray(range.category)).toBe(true);
    expect((range.category as string[]).length).toBeGreaterThan(10);
    expect((cfg.mark as Record<string, unknown>).color).toBeDefined();
  });

  it('uses a sequential gradient (not the categorical palette) for continuous color', () => {
    const cfg = buildVegaConfig(theme, { colorScheme: 'palette-classic' });
    const range = cfg.range as Record<string, unknown>;
    // ramp/heatmap must differ from the categorical palette so quantitative color
    // isn't a rainbow of categorical colors.
    expect(range.ramp).toBeDefined();
    expect(range.ramp).not.toEqual(range.category);
  });

  it('resolves Grafana continuous schemes to a color array', () => {
    const cfg = buildVegaConfig(theme, { colorScheme: 'continuous-GrYlRd' });
    const range = cfg.range as Record<string, unknown>;
    expect(Array.isArray(range.category)).toBe(true);
    expect((range.category as string[]).length).toBeGreaterThan(0);
  });

  it('treats the legacy grafana-classic alias as the classic palette', () => {
    const cfg = buildVegaConfig(theme, { colorScheme: 'grafana-classic' });
    expect(Array.isArray((cfg.range as Record<string, unknown>).category)).toBe(true);
  });

  it('uses a Vega color scheme by name for non-Grafana schemes', () => {
    const cfg = buildVegaConfig(theme, { colorScheme: 'tableau10' });
    const range = cfg.range as Record<string, Record<string, unknown>>;
    expect(range.category.scheme).toBe('tableau10');
    expect((cfg.mark as Record<string, unknown>).color).toBeUndefined();
  });
});

import { writeFileSync } from 'fs';
import { join } from 'path';

import { buildGalleries } from './galleries';
import examples from './__fixtures__/vegaLiteExamples.json';

describe('demo galleries', () => {
  const { files, stats } = buildGalleries(examples as Record<string, unknown>);

  it('builds builder-native galleries (no JSON-style options anywhere)', () => {
    expect(stats.total).toBeGreaterThan(400);
    const json = JSON.stringify(files);
    expect(json).not.toContain('specOverrideJson');
    expect(json).not.toContain('configJson');
    expect(json).not.toContain('advancedJson');
    console.log('GALLERY_STATS', JSON.stringify(stats));
  });

  it('writes the dashboards when GEN_GALLERIES=true', () => {
    if (process.env.GEN_GALLERIES !== 'true') {
      return;
    }
    const dir = join(__dirname, '../../provisioning/dashboards');
    for (const [file, dash] of Object.entries(files)) {
      writeFileSync(join(dir, file), JSON.stringify(dash, null, 2) + '\n');
    }
  });
});

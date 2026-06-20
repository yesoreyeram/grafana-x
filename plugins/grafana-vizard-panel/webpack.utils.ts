import * as fs from 'fs';
import * as path from 'path';

export function getPackageJson() {
  return require(path.resolve(process.cwd(), 'package.json'));
}

export function getPluginJson() {
  return require(path.resolve(process.cwd(), 'src/plugin.json'));
}

export function hasReadme() {
  return fs.existsSync(path.resolve(process.cwd(), 'src/README.md'));
}

// Entry points: the plugin module.
export async function getEntries(): Promise<Record<string, string>> {
  const parent = 'src';
  const entries: Record<string, string> = {};

  const moduleTs = path.resolve(process.cwd(), parent, 'module.ts');
  const moduleTsx = path.resolve(process.cwd(), parent, 'module.tsx');
  if (fs.existsSync(moduleTsx)) {
    entries['module'] = './module.tsx';
  } else if (fs.existsSync(moduleTs)) {
    entries['module'] = './module.ts';
  }

  return entries;
}

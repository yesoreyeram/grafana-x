module.exports = {
  testEnvironment: 'jsdom',
  testEnvironmentOptions: {
    // Pick browser builds for packages like `vega-canvas` whose Node build
    // uses top-level await (which Jest cannot parse). See exports map in
    // node_modules/vega-canvas/package.json.
    customExportConditions: ['browser', 'default'],
  },
  setupFilesAfterEnv: ['<rootDir>/jest-setup.js'],
  moduleNameMapper: {
    '\\.(css|scss|sass)$': 'identity-obj-proxy',
    // Force the browser build of vega-canvas; its Node build uses top-level
    // await (`await import('canvas')`) which Jest cannot parse.
    '^vega-canvas$': '<rootDir>/node_modules/vega-canvas/build/vega-canvas.browser.js',
  },
  transform: {
    '^.+\\.(t|j)sx?$': [
      '@swc/jest',
      {
        sourceMaps: true,
        jsc: {
          parser: { syntax: 'typescript', tsx: true, decorators: false },
          transform: { react: { runtime: 'automatic' } },
        },
      },
    ],
  },
  transformIgnorePatterns: [
    // Vega/Vega-Lite (and friends) ship ES modules only; let SWC transform them
    // so the examples coverage test can `compile()` specs in Node.
    'node_modules/(?!(@grafana|ol|d3|d3-.*|internmap|delaunator|robust-predicates|vega|vega-.*|fast-deep-equal|fast-json-patch|json-stringify-pretty-compact|clone)/)',
  ],
};

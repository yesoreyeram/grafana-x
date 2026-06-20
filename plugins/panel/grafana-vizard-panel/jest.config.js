module.exports = {
  testEnvironment: 'jsdom',
  setupFilesAfterEnv: ['<rootDir>/jest-setup.js'],
  moduleNameMapper: {
    '\\.(css|scss|sass)$': 'identity-obj-proxy',
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

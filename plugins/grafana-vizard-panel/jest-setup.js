// Jest setup provided by Grafana scaffolding
import '@testing-library/jest-dom';

// jsdom does not provide TextEncoder/TextDecoder which @grafana/ui (react-dom
// server) requires. Polyfill them from Node's util module.
import { TextEncoder, TextDecoder } from 'util';

if (typeof global.TextEncoder === 'undefined') {
  global.TextEncoder = TextEncoder;
}
if (typeof global.TextDecoder === 'undefined') {
  global.TextDecoder = TextDecoder;
}

// jsdom does not expose structuredClone, which Vega-Lite uses to clone specs.
// A JSON clone is sufficient for the JSON specs we test.
if (typeof global.structuredClone === 'undefined') {
  global.structuredClone = (value) => (value === undefined ? undefined : JSON.parse(JSON.stringify(value)));
}

// jsdom's canvas getContext is unimplemented; Vega touches it at import time for
// text measurement. Return a minimal 2D context stub to keep imports quiet.
if (typeof HTMLCanvasElement !== 'undefined') {
  HTMLCanvasElement.prototype.getContext = () => ({
    measureText: () => ({ width: 0 }),
    fillRect: () => {},
    clearRect: () => {},
    getImageData: () => ({ data: [] }),
    putImageData: () => {},
    createImageData: () => [],
    setTransform: () => {},
    drawImage: () => {},
    save: () => {},
    restore: () => {},
    beginPath: () => {},
    moveTo: () => {},
    lineTo: () => {},
    closePath: () => {},
    stroke: () => {},
    fill: () => {},
    translate: () => {},
    scale: () => {},
    rotate: () => {},
    arc: () => {},
    rect: () => {},
  });
}

// jsdom lacks matchMedia, which several @grafana/ui dependencies (uplot) need.
Object.defineProperty(global, 'matchMedia', {
  writable: true,
  value: (query) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: jest.fn(),
    removeListener: jest.fn(),
    addEventListener: jest.fn(),
    removeEventListener: jest.fn(),
    dispatchEvent: jest.fn(),
  }),
});

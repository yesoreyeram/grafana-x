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

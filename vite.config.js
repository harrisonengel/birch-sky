import { defineConfig } from 'vite';

const MARKET_PLATFORM_URL = process.env.MARKET_PLATFORM_URL || 'http://localhost:8080';
const HARNESS_URL = process.env.HARNESS_URL || 'http://localhost:8000';
const PREPPER_URL = process.env.PREPPER_URL || 'http://localhost:8002';

export default defineConfig({
  root: '.',
  publicDir: 'assets',
  server: {
    proxy: {
      '/api/v1': {
        target: MARKET_PLATFORM_URL,
        changeOrigin: true,
      },
      '/agent': {
        target: HARNESS_URL,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/agent/, '/api'),
      },
      '/api/prepper': {
        target: PREPPER_URL,
        changeOrigin: true,
      },
    },
  },
});

import path from "path"
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const isDev = mode === 'development'

  // Environment variables with defaults
  const devPort = parseInt(process.env.VITE_DEV_PORT || '5173')
  const apiProxyTarget = process.env.VITE_API_PROXY_TARGET || 'http://localhost:8080'
  const basePath = process.env.VITE_BASE_PATH || '/'

  return {
    plugins: [react()],
    base: basePath,
    server: {
      port: devPort,
      host: process.env.VITE_DEV_HOST || 'localhost',
      proxy: isDev ? {
        '/v1': {
          target: apiProxyTarget,
          changeOrigin: true,
          secure: false,
        },
        '/auth': {
          target: apiProxyTarget,
          changeOrigin: true,
          secure: false,
        },
      } : undefined,
    },
    build: {
      outDir: process.env.VITE_BUILD_OUT_DIR || 'dist',
      sourcemap: process.env.VITE_BUILD_SOURCEMAP === 'true',
    },
    esbuild: {
      drop: mode === 'production' ? ['console', 'debugger'] : [],
    },
    define: {
      __APP_VERSION__: JSON.stringify(process.env.npm_package_version || '0.1.x'),
      'import.meta.env.VITE_APP_VERSION': JSON.stringify(process.env.npm_package_version || '0.1.x'),
    },
    resolve: {
      alias: {
        "@": path.resolve(__dirname, "./src"),
      },
    },
  }
})

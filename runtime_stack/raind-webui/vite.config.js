import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const devApiTarget = process.env.RAIND_WEBUI_DEV_API_TARGET || 'http://127.0.0.1:18081'
const devHttps = ['1', 'true', 'yes', 'on'].includes(
  String(process.env.RAIND_WEBUI_DEV_HTTPS || '').toLowerCase()
)
const devHttpsCertPath = process.env.RAIND_WEBUI_DEV_TLS_CERT || '/etc/raind/cert/web/raindWeb.crt'
const devHttpsKeyPath = process.env.RAIND_WEBUI_DEV_TLS_KEY || '/etc/raind/cert/web/raindWeb.key'

const httpsOptions =
  devHttps && fs.existsSync(devHttpsCertPath) && fs.existsSync(devHttpsKeyPath)
    ? {
        cert: fs.readFileSync(devHttpsCertPath),
        key: fs.readFileSync(devHttpsKeyPath)
      }
    : undefined

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const topVersionPath = path.resolve(__dirname, '../../VERSION')
const topVersionRaw = fs.existsSync(topVersionPath)
  ? fs.readFileSync(topVersionPath, 'utf8').trim()
  : ''
const injectedVersion = process.env.VITE_RAIND_VERSION || (topVersionRaw ? `v${topVersionRaw}` : 'dev')

export default defineConfig({
  plugins: [vue()],
  define: {
    'import.meta.env.VITE_RAIND_VERSION': JSON.stringify(injectedVersion)
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    https: httpsOptions,
    proxy: {
      '/auth': {
        target: devApiTarget,
        changeOrigin: true,
        secure: false
      },
      '/api': {
        target: devApiTarget,
        changeOrigin: true,
        ws: true,
        secure: false,
      }
    }
  }
})

import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: '/_ui/',
  build: {
    outDir: 'dist',
  },
  server: {
    proxy: {
      '/_api': 'http://localhost:8080',
      '/mcp': 'http://localhost:8080',
    },
  },
})

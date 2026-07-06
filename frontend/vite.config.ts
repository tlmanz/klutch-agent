import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Wails serves the built assets from the embedded FS at the server root, so use
// Vite's default absolute base (/) - matching the Wails template. A relative
// base ('./') can fail to resolve inside the webview's custom origin.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})

import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 3001,
    proxy: {
      '/portal/v1': {
        target: process.env.VITE_PORTAL_API_URL || 'http://localhost:8082',
        changeOrigin: true,
      },
      '/admin/v1': {
        target: process.env.VITE_ADMIN_API_URL || 'http://localhost:8081',
        changeOrigin: true,
      },
    },
  },
});

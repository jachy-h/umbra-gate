import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  root: 'dashboard/app',
  base: '/dashboard/',
  plugins: [vue()],
  build: {
    outDir: '../dist',
    emptyOutDir: true
  }
});

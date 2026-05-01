import { defineConfig } from 'astro/config';
import svelte from '@astrojs/svelte';

export default defineConfig({
  site: 'https://manudiv16.github.io',
  base: '/scheduled-db',
  output: 'static',
  integrations: [svelte()],
});

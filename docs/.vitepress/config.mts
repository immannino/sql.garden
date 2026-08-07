import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'sql.garden',
  description: 'An infinite canvas for your data',
  base: '/',
  outDir: './.vitepress/dist',

  head: [
    ['link', { rel: 'icon', href: 'data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22><text y=%22.9em%22 font-size=%2290%22>🌱</text></svg>' }],

    ['script', {src: '//gc.zgo.at/count.js', 'data-goatcounter': 'https://sqlgarden.goatcounter.com/count', async: 'true' }]
  ],

  themeConfig: {
    logo: { src: '/logo.svg', width: 24, height: 24 },
    siteTitle: 'sql.garden',

    nav: [
      { text: 'Documentation', link: '/guide/introduction.md' },
      { text: 'Sandbox', link: 'https://sql.garden/sandbox' },
      { text: 'Download', link: 'https://github.com/immannino/sql.garden/releases/latest' },
      { text: 'Blog', link: '/blog' },
    ],

    sidebar: [
      {
        text: 'Getting Started',
        items: [
          { text: 'Introduction', link: '/guide/introduction' },
          { text: 'Installation', link: '/guide/installation' },
          { text: 'Quick Start', link: '/guide/quickstart' },
        ],
      },
      {
        text: 'Canvas',
        items: [
          { text: 'Nodes', link: '/guide/nodes' },
          { text: 'Connections', link: '/guide/connections' },
          { text: 'Import Data', link: '/guide/import' },
          { text: 'Educational / Exercise Packs', link: '/guide/educational' },
        ],
      },
      {
        text: 'MCP / AI',
        items: [
          { text: 'Overview', link: '/mcp/overview' },
          { text: 'Configuration', link: '/mcp/configuration' },
          { text: 'Tool Reference', link: '/mcp/tools' },
        ],
      },
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/immannino/sql.garden' },
    ],

    footer: {
      message: 'Released under the GPL v3 License.',
      copyright: 'Copyright © 2024–2026 Tony Mannino',
    },

    search: {
      provider: 'local',
    },
  },
})

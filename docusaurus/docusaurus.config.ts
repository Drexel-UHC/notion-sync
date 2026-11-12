import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

// The user docs for notion-sync — humans and agents. Single source of truth for user-facing
// information. The binary still ships its own AGENTS.md (internal/sync/agents.md.tmpl);
// the two evolve separately for now.

const config: Config = {
  title: 'notion-sync',
  tagline: 'Sync Notion databases and pages to local Markdown files with YAML frontmatter.',

  future: {
    v4: true,
  },

  // Project Pages URL: https://drexel-uhc.github.io/notion-sync/
  url: 'https://drexel-uhc.github.io',
  baseUrl: '/notion-sync/',
  trailingSlash: true,
  organizationName: 'Drexel-UHC',
  projectName: 'notion-sync',

  onBrokenLinks: 'throw',
  onBrokenAnchors: 'throw',

  markdown: {
    // `.md` is CommonMark, `.mdx` is MDX — prose here carries `<folder>` placeholders that
    // MDX would read as JSX.
    format: 'detect',
    hooks: {
      onBrokenMarkdownLinks: 'throw',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          routeBasePath: '/',
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/Drexel-UHC/notion-sync/tree/main/docusaurus/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  plugins: [
    // Writes llms.txt, llms-full.txt and a Markdown copy of every page (llmstxt.org) so
    // agents can read the docs without rendering HTML.
    ['docusaurus-plugin-llms', {generateMarkdownFiles: true}],
  ],

  themeConfig: {
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'notion-sync',
      items: [
        {
          href: 'https://github.com/Drexel-UHC/notion-sync/releases',
          label: 'Releases',
          position: 'right',
        },
        {
          href: 'https://github.com/Drexel-UHC/notion-sync',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'powershell', 'json', 'yaml'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;

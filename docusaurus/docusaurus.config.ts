import fs from 'node:fs';
import path from 'node:path';
import {themes as prismThemes} from 'prism-react-renderer';
import type {Config, Plugin} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

// The user docs for notion-sync — humans and agents. Single source of truth for user-facing
// information. The binary still ships its own AGENTS.md (internal/sync/agents.md.tmpl);
// the two evolve separately for now.

const url = 'https://drexel-uhc.github.io';
const baseUrl = '/notion-sync/';

// Doc versions, newest first, as `docs:version vX.Y.Z` writes them (same rule as
// salurbal-obt). Every version lives at its own permanent path, `/<version>/`; `docs/` is the
// unreleased working copy, served at `/next/`. The site root is only a redirect to the newest
// version — `/next/` until the first version is cut.
const versionsFile = path.join(__dirname, 'versions.json');
const versions: string[] = fs.existsSync(versionsFile)
  ? JSON.parse(fs.readFileSync(versionsFile, 'utf8'))
  : [];
const latest = versions[0];
const latestPath = latest ? `/${latest}/` : '/next/';

// Writes the site root: an absolute-URL redirect to the newest version.
function rootRedirect(): Plugin {
  return {
    name: 'root-redirect',
    async postBuild({outDir}) {
      const target = `${url}${baseUrl}${latestPath.slice(1)}`;
      const html = `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta http-equiv="refresh" content="0; url=${target}">
<link rel="canonical" href="${target}">
<title>notion-sync</title></head>
<body><a href="${target}">${target}</a></body></html>
`;
      fs.writeFileSync(path.join(outDir, 'index.html'), html);
    },
  };
}

const config: Config = {
  title: 'notion-sync',
  tagline: 'Sync Notion databases and pages to local Markdown files with YAML frontmatter.',
  favicon: 'img/logo.svg',

  future: {
    v4: true,
  },

  // Project Pages URL: https://drexel-uhc.github.io/notion-sync/
  url,
  baseUrl,
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
          lastVersion: latest ?? 'current',
          versions: {
            current: {label: 'next', path: 'next', banner: latest ? 'unreleased' : 'none'},
            ...Object.fromEntries(versions.map((v) => [v, {label: v, path: v, banner: 'none'}])),
          },
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  plugins: [
    rootRedirect,
    // Writes llms.txt, llms-full.txt and a Markdown copy of every page (llmstxt.org), one set
    // per version: /next/llms.txt for the working copy, /<version>/llms.txt for each release.
    [
      'docusaurus-plugin-llms',
      {
        generateMarkdownFiles: true,
        versions: [
          {name: 'next', docsDir: 'docs', path: 'next'},
          ...versions.map((v) => ({name: v, docsDir: `versioned_docs/version-${v}`, path: v})),
        ],
      },
    ],
  ],

  themeConfig: {
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'notion-sync',
      // The site root is a redirect, not a page, so the title links straight to the newest version.
      logo: {alt: 'notion-sync', src: 'img/logo.svg', href: latestPath},
      items: [
        {type: 'docsVersionDropdown', position: 'right'},
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

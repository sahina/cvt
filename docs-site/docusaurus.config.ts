import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Contract Validator Toolkit',
  tagline: 'Consumer-based contract validation for OpenAPI specifications',
  favicon: 'img/favicon.ico',

  // Production URL
  url: 'https://sahina.github.io',
  baseUrl: '/cvt/',

  // GitHub pages deployment config
  organizationName: 'sahina',
  projectName: 'cvt',
  deploymentBranch: 'gh-pages',
  trailingSlash: false,

  onBrokenLinks: 'throw',

  // Development server configuration
  customFields: {
    devServerPort: 4100,
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  // Markdown/MDX configuration
  markdown: {
    mermaid: true,
    format: 'detect', // Treat .md as markdown, .mdx as MDX (avoids JSX parsing issues)
  },
  themes: ['@docusaurus/theme-mermaid'],

  presets: [
    [
      'classic',
      {
        docs: {
          path: '../docs',
          routeBasePath: 'docs',
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/sahina/cvt/tree/main/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    // Social card image
    image: 'img/cvt-social-card.png',
    navbar: {
      title: 'CVT',
      logo: {
        alt: 'CVT Logo',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docs',
          position: 'left',
          label: 'Docs',
        },
        {
          to: '/docs/reference/api',
          label: 'API Reference',
          position: 'left',
        },
        {
          href: 'https://github.com/sahina/cvt',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      copyright: `Copyright © ${new Date().getFullYear()} CVT Project. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'go', 'java', 'python', 'protobuf', 'json'],
    },
    colorMode: {
      defaultMode: 'light',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    mermaid: {
      theme: {light: 'neutral', dark: 'dark'},
    },
  } satisfies Preset.ThemeConfig,

  plugins: [
    [
      require.resolve('@easyops-cn/docusaurus-search-local'),
      {
        hashed: true,
        docsRouteBasePath: '/docs',
        docsDir: '../docs',
        indexBlog: false,
        highlightSearchTermsOnTargetPage: true,
      },
    ],
  ],
};

export default config;

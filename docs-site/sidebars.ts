import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docs: [
    'intro',
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: [
        'getting-started/installation',
        'getting-started/quick-start',
      ],
    },
    {
      type: 'category',
      label: 'AI Helper',
      collapsed: true,
      items: [
        'ai-helper/overview',
        'ai-helper/context-templates',
        'ai-helper/common-mistakes',
        'ai-helper/advanced-patterns',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      collapsed: true,
      items: [
        'guides/consumer-testing',
        'guides/producer-testing',
        'guides/breaking-changes',
        'guides/validation-modes',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      collapsed: true,
      items: [
        'reference/api',
        'reference/cli',
        'reference/configuration',
        {
          type: 'category',
          label: 'SDKs',
          collapsed: true,
          items: [
            'reference/sdk/index',
            'reference/sdk/nodejs',
            'reference/sdk/python',
            'reference/sdk/go',
            'reference/sdk/java',
          ],
        },
        'reference/openapi-support',
      ],
    },
    {
      type: 'category',
      label: 'Operations',
      collapsed: true,
      items: [
        'operations/observability',
      ],
    },
    {
      type: 'category',
      label: 'Development',
      collapsed: true,
      items: [
        'development/contributing',
        'development/releasing',
      ],
    },
    {
      type: 'category',
      label: 'Internal',
      collapsed: true,
      items: [
        'internal/prd',
        'internal/adoption-strategy',
      ],
    },
  ],
};

export default sidebars;

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
        'getting-started/faq',
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
        'ai-helper/openapi-schema-generator',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      collapsed: true,
      items: [
        'guides/consumer-testing',
        'guides/producer-testing',
        'guides/can-i-deploy',
        'guides/breaking-changes',
        'guides/validation-modes',
        'guides/ci-cd-integration',
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
          label: 'Architecture',
          collapsed: true,
          items: [
            'reference/architecture/index',
            'reference/architecture/validation-engine',
            'reference/architecture/storage-layer',
            'reference/architecture/consumer-registry',
            'reference/architecture/sdk-architecture',
          ],
        },
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
      label: 'Plugins',
      collapsed: true,
      items: [
        'plugins/README',
        'plugins/config',
        'plugins/authoring-go',
        'plugins/reference-plugins',
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
  ],
};

export default sidebars;

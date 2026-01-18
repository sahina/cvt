import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docs: [
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: [
        'intro',
        'prd',
        'use-cases',
      ],
    },
    {
      type: 'category',
      label: 'Consumer Testing',
      collapsed: false,
      items: [
        'consumer-testing-guide',
        'consumer-testing-plan',
      ],
    },
    {
      type: 'category',
      label: 'Producer Testing',
      collapsed: false,
      items: [
        'producer-testing',
        'producer-testing-plan',
      ],
    },
    {
      type: 'category',
      label: 'Operations',
      collapsed: false,
      items: [
        'DEVELOPMENT',
        'OBSERVABILITY',
        'modes',
      ],
    },
    {
      type: 'category',
      label: 'Architecture',
      collapsed: true,
      items: [
        'sequence-diagrams',
      ],
    },
    {
      type: 'category',
      label: 'Strategy',
      collapsed: true,
      items: [
        'adoption-strategy',
      ],
    },
    'api-reference',
  ],
};

export default sidebars;

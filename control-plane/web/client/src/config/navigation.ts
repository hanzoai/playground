import type { NavigationSection } from '@/components/Navigation/types';

export const navigationSections: NavigationSection[] = [
  {
    id: 'main',
    title: '',
    items: [
      {
        id: 'launch',
        label: 'Launch',
        href: '/launch',
        icon: 'rocket',
        description: 'Launch a cloud agent'
      },
      {
        id: 'nodes',
        label: 'My Agents',
        href: '/nodes',
        icon: 'function',
        description: 'Running agents and connected nodes'
      },
      {
        id: 'playground',
        label: 'Playground',
        href: '/playground',
        icon: 'bot',
        description: 'Visual agent orchestration canvas'
      },
    ]
  },
  {
    id: 'work',
    title: '',
    items: [
      {
        id: 'control-plane',
        label: 'Control Plane',
        href: '/bots/all',
        icon: 'data-center',
        description: 'Orchestrate distributed agents'
      },
      {
        id: 'executions',
        label: 'Executions',
        href: '/executions',
        icon: 'run',
        description: 'Agent executions and runtime calls'
      },
      {
        id: 'workflows',
        label: 'Workflows',
        href: '/workflows',
        icon: 'flow-data',
        description: 'Multi-step workflow processes'
      },
      {
        id: 'teams',
        label: 'Teams',
        href: '/teams',
        icon: 'users',
        description: 'Provision and manage agent teams'
      },
    ]
  },
  {
    id: 'manage',
    title: '',
    items: [
      {
        id: 'network',
        label: 'Network',
        href: '/network',
        icon: 'network',
        description: 'AI capacity marketplace and earnings'
      },
      {
        id: 'marketplace',
        label: 'Marketplace',
        href: '/marketplace',
        icon: 'storefront',
        description: 'Buy and sell AI capacity',
      },
      {
        id: 'metrics',
        label: 'Metrics',
        href: '/metrics',
        icon: 'dashboard',
        description: 'System overview and operational metrics'
      },
      {
        id: 'organization',
        label: 'Organization',
        href: '/org/settings',
        icon: 'organization',
        description: 'Organization settings and member management'
      },
      {
        id: 'settings',
        label: 'Settings',
        href: '/settings',
        icon: 'settings',
        description: 'Gateway, identity, and observability'
      },
    ]
  }
];

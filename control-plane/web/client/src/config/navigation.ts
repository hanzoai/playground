import type { NavigationSection } from '@/components/Navigation/types';

export const navigationSections: NavigationSection[] = [
  {
    id: 'main',
    title: '',
    items: [
      {
        id: 'nodes',
        label: 'My Bots',
        href: '/nodes',
        icon: 'data-center',
        description: 'Manage local and cloud bots'
      },
      {
        id: 'playground',
        label: 'Playground',
        href: '/playground',
        icon: 'bot',
        description: 'Visual bot orchestration canvas'
      },
    ]
  },
  {
    id: 'work',
    title: '',
    items: [
      {
        id: 'executions',
        label: 'Executions',
        href: '/executions',
        icon: 'run',
        description: 'Bot executions and runtime calls'
      },
    ]
  },
  {
    id: 'manage',
    title: '',
    items: [
      {
        id: 'metrics',
        label: 'Metrics',
        href: '/metrics',
        icon: 'dashboard',
        description: 'System overview and operational metrics'
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

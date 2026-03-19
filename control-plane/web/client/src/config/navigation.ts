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
        icon: 'bot',
        description: 'Running bots and connected nodes'
      },
      {
        id: 'playground',
        label: 'Playground',
        href: '/playground',
        icon: 'playground',
        description: 'Visual bot orchestration canvas'
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
        description: 'Orchestrate distributed bots'
      },
      {
        id: 'executions',
        label: 'Executions',
        href: '/executions',
        icon: 'terminal',
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
        icon: 'analytics',
        description: 'System overview and operational metrics'
      },
      {
        id: 'organization',
        label: 'Organization',
        href: '/org/settings',
        icon: 'organization',
        description: 'Team members, invites, and org settings'
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

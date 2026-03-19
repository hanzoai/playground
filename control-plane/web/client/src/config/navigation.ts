import type { NavigationSection } from '@/components/Navigation/types';

export const navigationSections: NavigationSection[] = [
  {
    id: 'main',
    title: '',
    items: [
      {
        id: 'playground',
        label: 'Playground',
        href: '/playground',
        icon: 'playground',
        description: 'Visual bot orchestration canvas'
      },
      {
        id: 'bots',
        label: 'Bots',
        href: '/bots',
        icon: 'bot',
        description: 'Your bots and connected agents'
      },
      {
        id: 'tasks',
        label: 'Tasks',
        href: '/tasks',
        icon: 'terminal',
        description: 'Bot tasks and runtime activity'
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

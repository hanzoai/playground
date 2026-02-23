import type { NavigationSection } from '@/components/Navigation/types';

export const navigationSections: NavigationSection[] = [
  {
    id: 'overview',
    title: 'Overview',
    items: [
      {
        id: 'dashboard',
        label: 'Dashboard',
        href: '/dashboard',
        icon: 'dashboard',
        description: 'Real-time system overview and operational metrics'
      },
      {
        id: 'playground',
        label: 'Playground',
        href: '/playground',
        icon: 'bot',
        description: 'Visual bot orchestration playground'
      },
      {
        id: 'spaces',
        label: 'Spaces',
        href: '/spaces',
        icon: 'data-center',
        description: 'Project workspaces for bots, nodes, and teams'
      },
      {
        id: 'teams',
        label: 'Teams',
        href: '/teams',
        icon: 'users',
        description: 'Provision and manage bot teams'
      }
    ]
  },
  {
    id: 'bot-hub',
    title: 'Bot Hub',
    items: [
      {
        id: 'node-overview',
        label: 'Node',
        href: '/nodes',
        icon: 'data-center',
        description: 'Node infrastructure and status'
      },
      {
        id: 'all-bots',
        label: 'Bots',
        href: '/bots/all',
        icon: 'function',
        description: 'Browse and manage all bots'
      }
    ]
  },
  {
    id: 'executions',
    title: 'Executions',
    items: [
      {
        id: 'individual-executions',
        label: 'Individual Executions',
        href: '/executions',
        icon: 'run',
        description: 'Single bot executions and calls'
      },
      {
        id: 'workflow-executions',
        label: 'Workflow Executions',
        href: '/workflows',
        icon: 'flow-data',
        description: 'Multi-step workflow processes'
      }
    ]
  },
  {
    id: 'identity-trust',
    title: 'Identity & Trust',
    items: [
      {
        id: 'did-explorer',
        label: 'DID Explorer',
        href: '/identity/dids',
        icon: 'identification',
        description: 'Explore decentralized identifiers for bots and bots'
      },
      {
        id: 'credentials',
        label: 'Credentials',
        href: '/identity/credentials',
        icon: 'shield-check',
        description: 'View and verify execution credentials'
      }
    ]
  },
  {
    id: 'settings',
    title: 'Settings',
    items: [
      {
        id: 'gateway',
        label: 'Gateway',
        href: '/settings',
        icon: 'link',
        description: 'Connect to a local or remote bot gateway'
      },
      {
        id: 'observability-webhook',
        label: 'Observability Webhook',
        href: '/settings/observability-webhook',
        icon: 'settings',
        description: 'Configure external event forwarding'
      }
    ]
  }
];

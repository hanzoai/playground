import type { NavigationSection } from '@/components/Navigation/types';

export const navigationSections: NavigationSection[] = [
  {
    id: 'core',
    title: '',
    items: [
      {
        id: 'dashboard',
        label: 'Dashboard',
        href: '/dashboard',
        icon: 'dashboard',
        description: 'System overview and operational metrics'
      },
      {
        id: 'control-plane',
        label: 'Control Plane',
        href: '/bots/all',
        icon: 'data-center',
        description: 'Orchestrate distributed agents and runtime execution'
      },
      {
        id: 'playground',
        label: 'Playground',
        href: '/playground',
        icon: 'bot',
        description: 'Visual agent orchestration canvas'
      },
      {
        id: 'nodes',
        label: 'Nodes',
        href: '/nodes',
        icon: 'function',
        description: 'Agent node infrastructure and status'
      },
      {
        id: 'spaces',
        label: 'Spaces',
        href: '/spaces',
        icon: 'data-center',
        description: 'Project workspaces for agents and teams'
      },
      {
        id: 'teams',
        label: 'Teams',
        href: '/teams',
        icon: 'users',
        description: 'Provision and manage agent teams'
      }
    ]
  },
  {
    id: 'executions',
    title: '',
    items: [
      {
        id: 'individual-executions',
        label: 'Executions',
        href: '/executions',
        icon: 'run',
        description: 'Agent executions and runtime calls'
      },
      {
        id: 'workflow-executions',
        label: 'Workflows',
        href: '/workflows',
        icon: 'flow-data',
        description: 'Multi-step workflow processes'
      }
    ]
  },
  {
    id: 'identity-trust',
    title: '',
    items: [
      {
        id: 'did-explorer',
        label: 'DID Explorer',
        href: '/identity/dids',
        icon: 'identification',
        description: 'Decentralized identifiers for agents'
      },
      {
        id: 'credentials',
        label: 'Credentials',
        href: '/identity/credentials',
        icon: 'shield-check',
        description: 'Execution credentials and verification'
      }
    ]
  },
  {
    id: 'settings',
    title: '',
    items: [
      {
        id: 'gateway',
        label: 'Gateway',
        href: '/settings',
        icon: 'link',
        description: 'Agent gateway connection'
      },
      {
        id: 'observability-webhook',
        label: 'Observability',
        href: '/settings/observability-webhook',
        icon: 'settings',
        description: 'Event forwarding and observability'
      }
    ]
  }
];

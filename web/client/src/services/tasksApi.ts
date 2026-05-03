import { getGlobalApiKey, getGlobalIamToken } from './api';

const API_BASE = import.meta.env.VITE_API_BASE_URL || '';
const SPACE_ID = 'default';

function authHeaders(): HeadersInit {
  const token = getGlobalIamToken() || getGlobalApiKey();
  return {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
}

function tasksUrl(path = ''): string {
  return `${API_BASE}/spaces/${SPACE_ID}/tasks${path}`;
}

function workflowsUrl(path = ''): string {
  return `${API_BASE}/spaces/${SPACE_ID}/workflows${path}`;
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface Task {
  id: string;
  title: string;
  description?: string;
  status: 'pending' | 'claimed' | 'running' | 'completed' | 'failed' | 'cancelled';
  priority: 'low' | 'medium' | 'high' | 'critical';
  input?: any;
  output?: any;
  error?: string;
  assigned_to?: string;
  workflow_id?: string;
  progress?: number;
  timeout_ms?: number;
  max_retries?: number;
  created_at?: string;
  updated_at?: string;
  completed_at?: string;
}

export interface CreateTaskParams {
  title: string;
  description?: string;
  priority?: 'low' | 'medium' | 'high' | 'critical';
  input?: any;
  timeout_ms?: number;
  max_retries?: number;
  workflow_id?: string;
  target_bot?: string;
}

export interface Workflow {
  id: string;
  name: string;
  status: string;
  tasks: Task[];
  created_at?: string;
}

// ---------------------------------------------------------------------------
// API Functions
// ---------------------------------------------------------------------------

export async function listTasks(): Promise<Task[]> {
  try {
    const res = await fetch(tasksUrl(), { headers: authHeaders() });
    if (!res.ok) {
      console.error(`listTasks failed: ${res.status} ${res.statusText}`);
      return [];
    }
    const data = await res.json();
    return Array.isArray(data) ? data : data.tasks ?? [];
  } catch (err) {
    console.error('listTasks error:', err);
    return [];
  }
}

export async function getTask(taskId: string): Promise<Task> {
  const res = await fetch(tasksUrl(`/${taskId}`), { headers: authHeaders() });
  if (!res.ok) {
    throw new Error(`getTask failed: ${res.status} ${res.statusText}`);
  }
  return res.json();
}

export async function createTask(params: CreateTaskParams): Promise<Task> {
  const res = await fetch(tasksUrl(), {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify(params),
  });
  if (!res.ok) {
    const body = await res.text().catch(() => '');
    throw new Error(`createTask failed: ${res.status} ${res.statusText} – ${body}`);
  }
  return res.json();
}

export async function cancelTask(taskId: string): Promise<void> {
  const res = await fetch(tasksUrl(`/${taskId}`), {
    method: 'DELETE',
    headers: authHeaders(),
  });
  if (!res.ok) {
    throw new Error(`cancelTask failed: ${res.status} ${res.statusText}`);
  }
}

export async function retryTask(taskId: string): Promise<Task> {
  const original = await getTask(taskId);
  return createTask({
    title: original.title,
    description: original.description,
    priority: original.priority,
    input: original.input,
    timeout_ms: original.timeout_ms,
    max_retries: original.max_retries,
    workflow_id: original.workflow_id,
  });
}

export async function listWorkflows(): Promise<Workflow[]> {
  try {
    const res = await fetch(workflowsUrl(), { headers: authHeaders() });
    if (!res.ok) {
      console.error(`listWorkflows failed: ${res.status} ${res.statusText}`);
      return [];
    }
    const data = await res.json();
    return Array.isArray(data) ? data : data.workflows ?? [];
  } catch (err) {
    console.error('listWorkflows error:', err);
    return [];
  }
}

export async function createWorkflow(
  name: string,
  tasks: CreateTaskParams[],
): Promise<Workflow> {
  const res = await fetch(workflowsUrl(), {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ name, tasks }),
  });
  if (!res.ok) {
    const body = await res.text().catch(() => '');
    throw new Error(`createWorkflow failed: ${res.status} ${res.statusText} – ${body}`);
  }
  return res.json();
}

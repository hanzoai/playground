import type { BotsResponse, BotWithNode, BotFilters } from '../types/bots';
import type {
  ExecutionRequest,
  ExecutionResponse,
  ExecutionHistory,
  PerformanceMetrics,
  ExecutionTemplate,
  AsyncExecuteResponse,
  ExecutionStatusResponse
} from '../types/execution';
import { getGlobalApiKey } from './api';

const API_BASE_URL = '/api/ui/v1';
const withAuthHeaders = (headers?: HeadersInit) => {
  const merged = new Headers(headers || {});
  const apiKey = getGlobalApiKey();
  if (apiKey) {
    merged.set('X-API-Key', apiKey);
  }
  return merged;
};

export class BotsApiError extends Error {
  public status?: number;

  constructor(message: string, status?: number) {
    super(message);
    this.name = 'BotsApiError';
    this.status = status;
  }
}

export const botsApi = {
  /**
   * Fetch all bots with optional filters
   */
  getAllBots: async (filters: BotFilters = {}): Promise<BotsResponse> => {
    const params = new URLSearchParams();

    if (filters.status && filters.status !== 'all') {
      params.append('status', filters.status);
    }
    if (filters.search) {
      params.append('search', filters.search);
    }
    if (filters.limit) {
      params.append('limit', filters.limit.toString());
    }
    if (filters.offset) {
      params.append('offset', filters.offset.toString());
    }

    const url = `${API_BASE_URL}/bots/all${params.toString() ? `?${params.toString()}` : ''}`;

    try {
      const response = await fetch(url, { headers: withAuthHeaders() });

      if (!response.ok) {
        throw new BotsApiError(
          `Failed to fetch bots: ${response.statusText}`,
          response.status
        );
      }

      const data: BotsResponse = await response.json();

      // Validate and ensure proper structure
      const validatedData: BotsResponse = {
        bots: Array.isArray(data.bots) ? data.bots : [],
        total: typeof data.total === 'number' ? data.total : 0,
        online_count: typeof data.online_count === 'number' ? data.online_count : 0,
        offline_count: typeof data.offline_count === 'number' ? data.offline_count : 0,
        nodes_count: typeof data.nodes_count === 'number' ? data.nodes_count : 0,
      };

      return validatedData;
    } catch (error) {
      if (error instanceof BotsApiError) {
        throw error;
      }
      throw new BotsApiError(`Network error: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  },

  /**
   * Fetch details for a specific bot
   */
  getBotDetails: async (botId: string): Promise<BotWithNode> => {
    const url = `${API_BASE_URL}/bots/${encodeURIComponent(botId)}/details`;

    try {
      const response = await fetch(url, { headers: withAuthHeaders() });

      if (!response.ok) {
        if (response.status === 404) {
          throw new BotsApiError('Bot not found', 404);
        }
        throw new BotsApiError(
          `Failed to fetch bot details: ${response.statusText}`,
          response.status
        );
      }

      const data: BotWithNode = await response.json();
      return data;
    } catch (error) {
      if (error instanceof BotsApiError) {
        throw error;
      }
      throw new BotsApiError(`Network error: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  },

  /**
   * Execute a bot with given input data (synchronous - waits for completion)
   * Enhanced version with proper request/response types and validation
   */
  executeBot: async (botId: string, request: ExecutionRequest): Promise<ExecutionResponse> => {
    const url = `/api/v1/execute/${encodeURIComponent(botId)}`;

    try {
      const response = await fetch(url, {
        method: 'POST',
        headers: withAuthHeaders({
          'Content-Type': 'application/json',
        }),
        body: JSON.stringify(request),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new BotsApiError(
          `Failed to execute bot: ${response.statusText} - ${errorText}`,
          response.status
        );
      }

      const data: ExecutionResponse = await response.json();

      // Validate response structure
      if (!data || typeof data !== 'object') {
        console.error('Invalid response structure:', data);
        throw new BotsApiError('Invalid response format from server');
      }

      // Log response for debugging if it seems malformed
      if (!data.result && !data.error_message && data.status !== 'succeeded') {
        console.warn('Response missing both result and error_message fields:', data);
      }

      return data;
    } catch (error) {
      if (error instanceof BotsApiError) {
        throw error;
      }

      // Enhanced error logging for debugging
      console.error('Bot execution error:', {
        botId,
        request,
        error: error instanceof Error ? error.message : error
      });

      throw new BotsApiError(`Network error: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  },

  /**
   * Execute a bot asynchronously (returns immediately with execution_id)
   * Use this when you need the execution_id immediately for navigation/tracking
   */
  executeBotAsync: async (botId: string, request: ExecutionRequest): Promise<AsyncExecuteResponse> => {
    const url = `/api/v1/execute/async/${encodeURIComponent(botId)}`;

    try {
      const response = await fetch(url, {
        method: 'POST',
        headers: withAuthHeaders({
          'Content-Type': 'application/json',
        }),
        body: JSON.stringify(request),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new BotsApiError(
          `Failed to execute bot async: ${response.statusText} - ${errorText}`,
          response.status
        );
      }

      const data: AsyncExecuteResponse = await response.json();

      // Validate response structure
      if (!data || typeof data !== 'object' || !data.execution_id) {
        console.error('Invalid async response structure:', data);
        throw new BotsApiError('Invalid async response format from server');
      }

      return data;
    } catch (error) {
      if (error instanceof BotsApiError) {
        throw error;
      }

      // Enhanced error logging for debugging
      console.error('Bot async execution error:', {
        botId,
        request,
        error: error instanceof Error ? error.message : error
      });

      throw new BotsApiError(`Network error: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  },

  /**
   * Get execution status by execution_id
   * Use this to poll for execution completion after starting with executeBotAsync
   */
  getExecutionStatus: async (executionId: string): Promise<ExecutionStatusResponse> => {
    const url = `/api/v1/executions/${encodeURIComponent(executionId)}`;

    try {
      const response = await fetch(url, { headers: withAuthHeaders() });

      if (!response.ok) {
        if (response.status === 404) {
          throw new BotsApiError('Execution not found', 404);
        }
        throw new BotsApiError(
          `Failed to fetch execution status: ${response.statusText}`,
          response.status
        );
      }

      const data: ExecutionStatusResponse = await response.json();

      // Validate response structure
      if (!data || typeof data !== 'object' || !data.execution_id) {
        console.error('Invalid execution status response structure:', data);
        throw new BotsApiError('Invalid execution status response format from server');
      }

      return data;
    } catch (error) {
      if (error instanceof BotsApiError) {
        throw error;
      }

      console.error('Execution status fetch error:', {
        executionId,
        error: error instanceof Error ? error.message : error
      });

      throw new BotsApiError(`Network error: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  },

  /**
   * Get performance metrics for a specific bot
   */
  getPerformanceMetrics: async (botId: string): Promise<PerformanceMetrics> => {
    const url = `${API_BASE_URL}/bots/${encodeURIComponent(botId)}/metrics`;

    try {
      const response = await fetch(url, { headers: withAuthHeaders() });

      if (!response.ok) {
        throw new BotsApiError(
          `Failed to fetch performance metrics: ${response.statusText}`,
          response.status
        );
      }

      const data: PerformanceMetrics = await response.json();
      return data;
    } catch (error) {
      if (error instanceof BotsApiError) {
        throw error;
      }
      throw new BotsApiError(`Network error: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  },

  /**
   * Get execution history for a specific bot
   */
  getExecutionHistory: async (
    botId: string,
    page: number = 1,
    limit: number = 20
  ): Promise<ExecutionHistory> => {
    const params = new URLSearchParams({
      page: page.toString(),
      limit: limit.toString(),
    });

    const url = `${API_BASE_URL}/bots/${encodeURIComponent(botId)}/executions?${params.toString()}`;

    try {
      const response = await fetch(url, { headers: withAuthHeaders() });

      if (!response.ok) {
        throw new BotsApiError(
          `Failed to fetch execution history: ${response.statusText}`,
          response.status
        );
      }

      const data: ExecutionHistory = await response.json();
      return data;
    } catch (error) {
      if (error instanceof BotsApiError) {
        throw error;
      }
      throw new BotsApiError(`Network error: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  },

  /**
   * Get saved execution templates for a bot
   */
  getExecutionTemplates: async (botId: string): Promise<ExecutionTemplate[]> => {
    const url = `${API_BASE_URL}/bots/${encodeURIComponent(botId)}/templates`;

    try {
      const response = await fetch(url, { headers: withAuthHeaders() });

      if (!response.ok) {
        throw new BotsApiError(
          `Failed to fetch execution templates: ${response.statusText}`,
          response.status
        );
      }

      const data: ExecutionTemplate[] = await response.json();
      return data;
    } catch (error) {
      if (error instanceof BotsApiError) {
        throw error;
      }
      throw new BotsApiError(`Network error: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  },

  /**
   * Save an execution template
   */
  saveExecutionTemplate: async (
    botId: string,
    template: Omit<ExecutionTemplate, 'id' | 'created_at'>
  ): Promise<ExecutionTemplate> => {
    const url = `${API_BASE_URL}/bots/${encodeURIComponent(botId)}/templates`;

    try {
      const response = await fetch(url, {
        method: 'POST',
        headers: withAuthHeaders({
          'Content-Type': 'application/json',
        }),
        body: JSON.stringify(template),
      });

      if (!response.ok) {
        throw new BotsApiError(
          `Failed to save execution template: ${response.statusText}`,
          response.status
        );
      }

      const data: ExecutionTemplate = await response.json();
      return data;
    } catch (error) {
      if (error instanceof BotsApiError) {
        throw error;
      }
      throw new BotsApiError(`Network error: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  },

  /**
   * Create an SSE connection for real-time bot events
   */
  createEventStream: (
    onEvent: (event: any) => void,
    onError?: (error: Error) => void,
    onConnect?: () => void
  ): EventSource => {
    const apiKey = getGlobalApiKey();
    const url = apiKey
      ? `${API_BASE_URL}/bots/events?api_key=${encodeURIComponent(apiKey)}`
      : `${API_BASE_URL}/bots/events`;
    console.log('🔄 Attempting to create SSE connection to:', url);
    const eventSource = new EventSource(url);

    eventSource.onopen = () => {
      console.log('✅ Bot SSE connection opened successfully');
      onConnect?.();
    };

    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        console.log('📡 Received bot event:', data);
        onEvent(data);
      } catch (error) {
        console.error('❌ Failed to parse SSE event data:', error, 'Raw data:', event.data);
        onError?.(new Error('Failed to parse event data'));
      }
    };

    eventSource.onerror = (error) => {
      console.error('❌ Bot SSE connection error:', error);
      console.error('❌ EventSource readyState:', eventSource.readyState);
      console.error('❌ EventSource url:', eventSource.url);
      onError?.(new Error(`SSE connection error - readyState: ${eventSource.readyState}`));
    };

    return eventSource;
  },

  /**
   * Close an SSE connection
   */
  closeEventStream: (eventSource: EventSource): void => {
    if (eventSource) {
      eventSource.close();
      console.log('🔌 Bot SSE connection closed');
    }
  }
};

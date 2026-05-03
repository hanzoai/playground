/**
 * Settings Store (Zustand)
 *
 * Manages user-configurable settings like custom gateway URL/token.
 * Reads from/writes to localStorage for persistence across sessions.
 * Used by useGateway to allow connecting to a custom gateway (e.g. via tunnel).
 */

import { create } from 'zustand';

const STORAGE_GATEWAY_URL_KEY = 'hanzo_gateway_url';
const STORAGE_GATEWAY_TOKEN_KEY = 'hanzo_gateway_token';

interface SettingsState {
  gatewayUrl: string | null;
  gatewayToken: string | null;
  setGatewayUrl: (url: string | null) => void;
  setGatewayToken: (token: string | null) => void;
  reset: () => void;
}

export const useSettingsStore = create<SettingsState>((set) => ({
  gatewayUrl: (() => {
    try { return localStorage.getItem(STORAGE_GATEWAY_URL_KEY); } catch { return null; }
  })(),
  gatewayToken: (() => {
    try { return localStorage.getItem(STORAGE_GATEWAY_TOKEN_KEY); } catch { return null; }
  })(),

  setGatewayUrl: (url) => {
    set({ gatewayUrl: url });
    try {
      if (url) localStorage.setItem(STORAGE_GATEWAY_URL_KEY, url);
      else localStorage.removeItem(STORAGE_GATEWAY_URL_KEY);
    } catch { /* ok */ }
  },

  setGatewayToken: (token) => {
    set({ gatewayToken: token });
    try {
      if (token) localStorage.setItem(STORAGE_GATEWAY_TOKEN_KEY, token);
      else localStorage.removeItem(STORAGE_GATEWAY_TOKEN_KEY);
    } catch { /* ok */ }
  },

  reset: () => {
    set({ gatewayUrl: null, gatewayToken: null });
    try {
      localStorage.removeItem(STORAGE_GATEWAY_URL_KEY);
      localStorage.removeItem(STORAGE_GATEWAY_TOKEN_KEY);
    } catch { /* ok */ }
  },
}));

export const GATE_URL = process.env.APP_URL || 'http://panel.e2e.sk.localbox';
export const PANEL_URL = process.env.PANEL_URL || 'http://127.0.0.1:7401';
export const METRICS_URL = process.env.METRICS_URL || 'http://127.0.0.1:7402';

export const BASE_PATH = '/_system/starterkit';

export const PANEL_COMMIT = process.env.PANEL_COMMIT || '';

export const ADMIN = { email: 'admin@example.com', password: 'passpass' };
export const USER = { email: 'user@example.com', password: 'passpass' };

export const ADMIN_STATE_PATH = 'e2e/.cache/admin-state.json';
export const USER_STATE_PATH = 'e2e/.cache/user-state.json';

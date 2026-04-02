// API 调用封装
const API = (() => {
  const BASE = '/api/v1';

  function getKey() {
    return localStorage.getItem('api_key') || '';
  }

  async function request(method, path, body, options = {}) {
    const headers = { 'Content-Type': 'application/json' };
    const key = getKey();
    if (key) headers['Authorization'] = 'Bearer ' + key;

    const opts = { method, headers };
    if (body !== undefined) opts.body = JSON.stringify(body);

    const res = await fetch(BASE + path, opts);

    if (res.status === 401) {
      localStorage.removeItem('api_key');
      localStorage.removeItem('user');
      // Only redirect to login if this was an authenticated request
      if (!options.public) {
        window.location.hash = '#/login';
      }
      throw new Error('未授权，请登录');
    }
    if (res.status === 429) {
      throw new Error('请求过于频繁，请稍后再试');
    }

    let data;
    try { data = await res.json(); } catch { data = {}; }

    if (!res.ok) {
      throw new Error(data.error || data.message || `请求失败 (${res.status})`);
    }
    return data;
  }

  return {
    get: (path, opts) => request('GET', path, undefined, opts),
    post: (path, body, opts) => request('POST', path, body, opts),
    patch: (path, body, opts) => request('PATCH', path, body, opts),
    delete: (path, opts) => request('DELETE', path, undefined, opts),
    getDashboardStats: () => request('GET', '/admin/dashboard/stats'),
    getDashboardTrends: (days) => request('GET', `/admin/dashboard/trends?days=${days}`),
    getRecentReviews: (limit) => request('GET', `/admin/dashboard/recent-reviews?limit=${limit}`),
    // Relay
    getRelayInstances: () => request('GET', '/relay/instances'),
    createRelayInstance: (body) => request('POST', '/relay/instances', body),
    deleteRelayInstance: (id) => request('DELETE', '/relay/instances/' + id),
    testRelayConnection: (body) => request('POST', '/relay/instances/test-connection', body),
    regenerateRelaySecret: (id) => request('POST', '/relay/instances/' + id + '/regenerate-secret'),
    regenerateRelayWebhookSecret: (id) => request('POST', '/relay/instances/' + id + '/regenerate-webhook-secret'),
    testRelayWebhook: (id) => request('POST', '/relay/instances/' + id + '/test-webhook'),
    // Relay Webhook Keys
    listRelayKeys: (instanceId) => request('GET', '/relay/instances/' + instanceId + '/keys'),
    createRelayKey: (instanceId, body) => request('POST', '/relay/instances/' + instanceId + '/keys', body),
    deleteRelayKey: (instanceId, keyId) => request('DELETE', '/relay/instances/' + instanceId + '/keys/' + keyId),
    toggleRelayKey: (instanceId, keyId, body) => request('PATCH', '/relay/instances/' + instanceId + '/keys/' + keyId, body),
  };
})();

// API 调用封装
const API = (() => {
  const BASE = '/api/v1';

  function getKey() {
    return localStorage.getItem('api_key') || '';
  }

  async function request(method, path, body) {
    const headers = { 'Content-Type': 'application/json' };
    const key = getKey();
    if (key) headers['Authorization'] = 'Bearer ' + key;

    const opts = { method, headers };
    if (body !== undefined) opts.body = JSON.stringify(body);

    const res = await fetch(BASE + path, opts);

    if (res.status === 401) {
      localStorage.removeItem('api_key');
      localStorage.removeItem('user');
      window.location.hash = '#/login';
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
    get: (path) => request('GET', path),
    post: (path, body) => request('POST', path, body),
    patch: (path, body) => request('PATCH', path, body),
    delete: (path) => request('DELETE', path),
  };
})();

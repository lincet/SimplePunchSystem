// 後端 API 呼叫包裝。前端與後端同源（Netlify 會把 /api/* 轉發到 Render）。

export class ApiError extends Error {
  constructor(status, code, message) {
    super(message || '發生錯誤');
    this.status = status;
    this.code = code;
  }
}

const listeners = new Set();
/** 收到 401 / 需要改密碼時通知 app 換頁 */
export function onAuthProblem(fn) { listeners.add(fn); }

async function request(method, path, { json, form, timeout = 30000 } = {}) {
  const headers = { 'X-Requested-With': 'fetch' };
  let body;
  if (json !== undefined) {
    headers['Content-Type'] = 'application/json';
    body = JSON.stringify(json);
  } else if (form) {
    body = form;
  }
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), timeout);
  let res;
  try {
    res = await fetch(path, { method, headers, body, credentials: 'same-origin', signal: ctrl.signal });
  } catch (e) {
    throw new ApiError(0, 'network', e.name === 'AbortError' ? '連線逾時，請再試一次' : '無法連線到伺服器，請檢查網路');
  } finally {
    clearTimeout(timer);
  }
  const type = res.headers.get('Content-Type') || '';
  const data = type.includes('application/json') ? await res.json().catch(() => null) : null;
  if (!res.ok) {
    const err = new ApiError(res.status, data?.error || 'http_' + res.status,
      data?.message || (res.status >= 500 ? '伺服器暫時無法回應，請稍後再試' : '發生錯誤'));
    if (res.status === 401 && !path.startsWith('/api/auth/login')) listeners.forEach((fn) => fn('unauthorized'));
    if (err.code === 'password_change_required') listeners.forEach((fn) => fn('password'));
    throw err;
  }
  return data;
}

export const api = {
  get: (path, opts) => request('GET', path, opts),
  post: (path, json) => request('POST', path, { json }),
  put: (path, json) => request('PUT', path, { json }),
  upload: (path, form) => request('POST', path, { form, timeout: 60000 }),

  /** 下載檔案（匯出）：用 fetch 取得 blob 再觸發下載，iOS 加入主畫面模式也能用。 */
  async download(path, fallbackName) {
    const res = await fetch(path, { credentials: 'same-origin' });
    if (!res.ok) {
      const data = await res.json().catch(() => null);
      throw new ApiError(res.status, data?.error, data?.message || '下載失敗');
    }
    const cd = res.headers.get('Content-Disposition') || '';
    const m = cd.match(/filename\*=UTF-8''([^;]+)/);
    const name = m ? decodeURIComponent(m[1]) : fallbackName;
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = name;
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 60000);
  },
};

/** 等待後端醒來（Render 免費方案閒置會休眠）。onWaiting 在第一次失敗時呼叫。 */
export async function waitForServer(onWaiting, { maxWaitMs = 180000 } = {}) {
  const start = Date.now();
  let notified = false;
  for (;;) {
    try {
      return await request('GET', '/api/health', { timeout: 10000 });
    } catch (e) {
      if (!notified) { notified = true; onWaiting?.(); }
      if (Date.now() - start > maxWaitMs) throw e;
      await new Promise((r) => setTimeout(r, 3000));
    }
  }
}

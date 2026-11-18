// App 進入點：喚醒後端、載入登入狀態、hash 路由與版面。
import { api, onAuthProblem, waitForServer } from './api.js';
import { html, raw, setTimeZone, toast } from './util.js';
import loginView from './views/login.js';
import changePasswordView from './views/change-password.js';
import punchView from './views/punch.js';
import myRecordsView from './views/my-records.js';
import employeesView from './views/admin-employees.js';
import employeeFormView from './views/admin-employee-form.js';
import attendanceView from './views/admin-attendance.js';
import hoursView from './views/admin-hours.js';
import exportView from './views/admin-export.js';

const ICONS = {
  punch: '<path d="M12 7v5l3 2"/><circle cx="12" cy="12" r="9"/>',
  records: '<path d="M8 6h12M8 12h12M8 18h12"/><circle cx="4" cy="6" r="1"/><circle cx="4" cy="12" r="1"/><circle cx="4" cy="18" r="1"/>',
  people: '<circle cx="9" cy="8" r="3.5"/><path d="M2.5 20c.8-3.6 3.4-5.5 6.5-5.5s5.7 1.9 6.5 5.5"/><path d="M16 4.6a3.5 3.5 0 0 1 0 6.8M18 14.8c1.9.7 3 2.4 3.5 5.2"/>',
  calendar: '<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M3 10h18M8 3v4M16 3v4"/>',
  chart: '<path d="M4 20V10M10 20V4M16 20v-7M22 20H2"/>',
  download: '<path d="M12 4v11M7 10l5 5 5-5M4 20h16"/>',
};
const icon = (name) => `<svg viewBox="0 0 24 24" aria-hidden="true">${ICONS[name]}</svg>`;

const TABS = {
  employee: [
    { href: '#/punch', label: '打卡', icon: 'punch', key: 'punch' },
    { href: '#/records', label: '我的紀錄', icon: 'records', key: 'records' },
  ],
  admin: [
    { href: '#/admin/employees', label: '員工', icon: 'people', key: 'employees' },
    { href: '#/admin/attendance', label: '出勤', icon: 'calendar', key: 'attendance' },
    { href: '#/admin/hours', label: '工時', icon: 'chart', key: 'hours' },
    { href: '#/admin/export', label: '匯出', icon: 'download', key: 'export' },
  ],
};

// access: public | any（已登入，含尚未改密碼）| employee | admin
const ROUTES = [
  { path: '/login', view: loginView, access: 'public' },
  { path: '/change-password', view: changePasswordView, access: 'any' },
  { path: '/punch', view: punchView, access: 'employee' },
  { path: '/records', view: myRecordsView, access: 'employee' },
  { path: '/admin/employees', view: employeesView, access: 'admin' },
  { path: '/admin/employees/new', view: employeeFormView, access: 'admin' },
  { path: '/admin/employees/:id', view: employeeFormView, access: 'admin' },
  { path: '/admin/attendance', view: attendanceView, access: 'admin' },
  { path: '/admin/hours', view: hoursView, access: 'admin' },
  { path: '/admin/export', view: exportView, access: 'admin' },
];

const root = document.getElementById('app');
const ctx = {
  me: null,
  health: null,
  go(hash) {
    if (location.hash === hash) route();
    else location.hash = hash;
  },
  setMe(me) { ctx.me = me; },
  async logout() {
    try { await api.post('/api/auth/logout'); } catch { /* 已失效也沒關係 */ }
    ctx.me = null;
    ctx.go('#/login');
  },
  home() {
    if (!ctx.me) return '#/login';
    if (ctx.me.mustChangePassword) return '#/change-password';
    return ctx.me.role === 'admin' ? '#/admin/employees' : '#/punch';
  },
};

let cleanup = null;

function match(path) {
  for (const r of ROUTES) {
    const a = r.path.split('/');
    const b = path.split('/');
    if (a.length !== b.length) continue;
    const params = {};
    if (a.every((seg, i) => (seg.startsWith(':') ? ((params[seg.slice(1)] = decodeURIComponent(b[i])), true) : seg === b[i]))) {
      return { ...r, params };
    }
  }
  return null;
}

function parseHash() {
  const h = location.hash.replace(/^#/, '') || '/';
  const [path, qs] = h.split('?');
  return { path, query: new URLSearchParams(qs || '') };
}

async function route() {
  const { path, query } = parseHash();
  const m = match(path);
  if (!m) return ctx.go(ctx.home());

  if (m.access !== 'public' && !ctx.me) return ctx.go('#/login');
  if (m.access === 'public' && ctx.me) return ctx.go(ctx.home());
  if (ctx.me?.mustChangePassword && m.access !== 'any') return ctx.go('#/change-password');
  if ((m.access === 'employee' || m.access === 'admin') && ctx.me.role !== m.access) return ctx.go(ctx.home());

  if (typeof cleanup === 'function') cleanup();
  cleanup = null;
  window.scrollTo(0, 0);

  const view = m.view;
  const tabs = ctx.me && !ctx.me.mustChangePassword ? TABS[ctx.me.role] : null;
  const title = typeof view.title === 'function' ? view.title(m.params) : view.title;
  root.innerHTML = String(html`
    ${view.bare ? '' : html`
      <header class="topbar">
        ${view.back ? html`<a class="topbar-back" href="${view.back}" aria-label="返回">‹</a>` : ''}
        <h1>${title}</h1>
        ${ctx.me ? html`<button class="topbar-action" type="button" data-logout>登出</button>` : ''}
      </header>`}
    <main class="page ${view.bare ? 'page-bare' : ''} ${tabs ? 'has-tabs' : ''}"></main>
    ${tabs ? html`
      <nav class="tabbar">
        ${tabs.map((t) => html`
          <a href="${t.href}" class="${t.key === view.tab ? 'active' : ''}">
            ${raw(icon(t.icon))}<span>${t.label}</span>
          </a>`)}
      </nav>` : ''}`);
  root.querySelector('[data-logout]')?.addEventListener('click', () => {
    if (confirm('確定要登出嗎？')) ctx.logout();
  });
  const main = root.querySelector('main');
  try {
    cleanup = await view.render(main, { ...ctx, params: m.params, query });
  } catch (e) {
    if (e?.status === 401 || e?.code === 'password_change_required') return;
    main.innerHTML = String(html`<div class="empty"><p>${e.message || '載入失敗'}</p>
      <button class="btn btn-secondary" type="button" data-retry>重新載入</button></div>`);
    main.querySelector('[data-retry]').addEventListener('click', route);
  }
}

onAuthProblem((kind) => {
  if (kind === 'unauthorized') {
    if (ctx.me) toast('登入已過期，請重新登入', 'error');
    ctx.me = null;
    ctx.go('#/login');
  } else if (kind === 'password') {
    if (ctx.me) ctx.me.mustChangePassword = true;
    ctx.go('#/change-password');
  }
});

function splash(message, sub = '') {
  root.innerHTML = String(html`
    <div class="splash">
      <div class="splash-logo">${raw(icon('punch'))}</div>
      <p class="splash-msg">${message}</p>
      ${sub ? html`<p class="splash-sub">${sub}</p>` : ''}
      <div class="spinner"></div>
    </div>`);
}

async function boot() {
  splash('連線中…');
  try {
    ctx.health = await waitForServer(() =>
      splash('伺服器喚醒中…', '示範主機閒置後會休眠，第一次開啟約需 30–60 秒，請稍候。'));
  } catch {
    root.innerHTML = String(html`
      <div class="splash">
        <p class="splash-msg">無法連線到伺服器</p>
        <p class="splash-sub">請確認網路連線後再試一次。</p>
        <button class="btn btn-primary" type="button" id="retry">重試</button>
      </div>`);
    document.getElementById('retry').addEventListener('click', boot);
    return;
  }
  setTimeZone(ctx.health.timezone);
  try {
    ctx.me = await api.get('/api/me');
  } catch {
    ctx.me = null;
  }
  window.addEventListener('hashchange', route);
  route();
}

if ('serviceWorker' in navigator && location.protocol === 'https:') {
  navigator.serviceWorker.register('/sw.js').catch(() => {});
}

boot();

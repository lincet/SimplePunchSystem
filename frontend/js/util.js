// 共用小工具：HTML 跳脫、時間格式化、提示訊息。

class Raw {
  constructor(s) { this.s = s; }
  toString() { return this.s; }
}

/** 標記為不需跳脫的 HTML。 */
export const raw = (s) => new Raw(String(s));

export function esc(v) {
  return String(v ?? '').replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));
}

/** 安全的 HTML 樣板：插入值預設會跳脫，陣列會串接，Raw 不跳脫。 */
export function html(strings, ...vals) {
  const out = strings.map((s, i) => {
    if (i >= vals.length) return s;
    return s + render(vals[i]);
  });
  return raw(out.join(''));
}

function render(v) {
  if (v == null || v === false) return '';
  if (v instanceof Raw) return v.s;
  if (Array.isArray(v)) return v.map(render).join('');
  return esc(v);
}

export const $ = (sel, root = document) => root.querySelector(sel);
export const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];

// ---------- 時間 ----------

let tz = 'Asia/Taipei';
export function setTimeZone(name) { if (name) tz = name; }
export function timeZone() { return tz; }

function parts(d) {
  const f = new Intl.DateTimeFormat('en-CA', {
    timeZone: tz, year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23', weekday: 'short',
  });
  const o = {};
  for (const p of f.formatToParts(d)) o[p.type] = p.value;
  return o;
}

const WEEK = { Sun: '日', Mon: '一', Tue: '二', Wed: '三', Thu: '四', Fri: '五', Sat: '六' };

export function fmtTime(v, withSeconds = false) {
  if (!v) return '--:--';
  const p = parts(new Date(v));
  return withSeconds ? `${p.hour}:${p.minute}:${p.second}` : `${p.hour}:${p.minute}`;
}

/** YYYY-MM-DD（公司時區） */
export function dateStr(v = new Date()) {
  const p = parts(new Date(v));
  return `${p.year}-${p.month}-${p.day}`;
}

/** 2026/11/18（三） */
export function fmtDateLong(v) {
  const p = parts(typeof v === 'string' && v.length === 10 ? new Date(`${v}T12:00:00Z`) : new Date(v));
  return `${p.year}/${p.month}/${p.day}（${WEEK[p.weekday]}）`;
}

/** 11/18（三） */
export function fmtDateShort(ymd) {
  const p = parts(new Date(`${ymd}T12:00:00Z`));
  return `${p.month}/${p.day}（${WEEK[p.weekday]}）`;
}

export function weekdayOf(ymd) {
  return WEEK[parts(new Date(`${ymd}T12:00:00Z`)).weekday];
}

/** 對 YYYY-MM-DD 加減天數 */
export function addDays(ymd, n) {
  const d = new Date(`${ymd}T12:00:00Z`);
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

export function addMonths(ymd, n) {
  const [y, m] = ymd.split('-').map(Number);
  const d = new Date(Date.UTC(y, m - 1 + n, 1, 12));
  return d.toISOString().slice(0, 10);
}

export function monthStart(ymd) { return `${ymd.slice(0, 7)}-01`; }
export function monthEnd(ymd) { return addDays(addMonths(monthStart(ymd), 1), -1); }

/** 8.5 → "8 小時 30 分" */
export function fmtHours(h) {
  const totalMin = Math.round((h || 0) * 60);
  const hh = Math.floor(totalMin / 60);
  const mm = totalMin % 60;
  if (hh === 0) return `${mm} 分`;
  return mm ? `${hh} 小時 ${mm} 分` : `${hh} 小時`;
}

export function fmtDuration(ms) {
  return fmtHours(Math.max(0, ms) / 3600000);
}

export const GENDER = { male: '男', female: '女', other: '其他' };

// ---------- UI ----------

let toastTimer;
export function toast(msg, type = 'info') {
  let el = document.getElementById('toast');
  if (!el) {
    el = document.createElement('div');
    el.id = 'toast';
    el.setAttribute('role', 'status');
    document.body.appendChild(el);
  }
  el.className = `toast toast-${type} show`;
  el.textContent = msg;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.remove('show'), 3200);
}

/** 全螢幕看照片 */
export function showPhoto(url, caption = '') {
  const el = document.createElement('div');
  el.className = 'lightbox';
  el.innerHTML = String(html`
    <figure>
      <img src="${url}" alt="打卡照片">
      ${caption ? html`<figcaption>${caption}</figcaption>` : ''}
    </figure>
    <button class="lightbox-close" aria-label="關閉">✕</button>`);
  el.addEventListener('click', () => el.remove());
  document.body.appendChild(el);
}

/** 讓按鈕在非同步工作期間顯示忙碌狀態 */
export async function busy(btn, fn) {
  if (btn.disabled) return;
  const label = btn.innerHTML;
  btn.disabled = true;
  btn.classList.add('is-busy');
  try {
    return await fn();
  } finally {
    btn.disabled = false;
    btn.classList.remove('is-busy');
    btn.innerHTML = label;
  }
}

export function randomPassword(n = 8) {
  const chars = 'abcdefghjkmnpqrstuvwxyz23456789';
  const buf = new Uint32Array(n);
  crypto.getRandomValues(buf);
  return [...buf].map((x) => chars[x % chars.length]).join('');
}

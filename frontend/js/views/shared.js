// 多個頁面共用的畫面片段。
import { html, fmtTime, fmtHours, dateStr } from '../util.js';

const photo = (id) => `/api/photos/${id}`;

function stamp(type, at, id, date, open) {
  if (!at) {
    return open
      ? html`<div class="stamp stamp-missing"><span class="tag tag-in">上班中</span></div>`
      : html`<div class="stamp stamp-missing"><span class="tag tag-warn">未打下班卡</span></div>`;
  }
  const label = type === 'in' ? '上班' : '下班';
  const t = fmtTime(at);
  const crossDay = date && dateStr(at) !== date;
  return html`
    <div class="stamp">
      <button type="button" class="thumb" data-photo="${photo(id)}" data-caption="${label} ${t}">
        <img src="${photo(id)}" alt="${label}照片" loading="lazy">
      </button>
      <div>
        <span class="tag tag-${type}">${label}</span>
        <time>${t}</time>${crossDay ? html`<small class="muted">（隔日）</small>` : ''}
      </div>
    </div>`;
}

/** 一天內的上班時段列表：上班照片/時間 → 下班照片/時間・時數 */
export function segmentRows(segments) {
  return html`
    <ul class="segments">
      ${segments.map((s) => html`
        <li class="segment ${s.complete || s.open ? '' : 'is-incomplete'}">
          ${stamp('in', s.in, s.inId)}
          <span class="arrow" aria-hidden="true">→</span>
          ${stamp('out', s.out, s.outId, s.date, s.open)}
          <span class="seg-hours">${s.complete ? fmtHours(s.minutes / 60) : '—'}</span>
        </li>`)}
    </ul>`;
}

/** 員工下拉選單 */
export function employeeSelect(users, selected, { all = true, name = 'user' } = {}) {
  return html`
    <select name="${name}">
      ${all ? html`<option value="">全部員工</option>` : ''}
      ${users.map((u) => html`<option value="${u.id}" ${String(u.id) === String(selected) ? 'selected' : ''}>
        ${u.name}${u.active ? '' : '（已停用）'}</option>`)}
    </select>`;
}

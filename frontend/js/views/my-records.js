import { api } from '../api.js';
import { html, fmtDateShort, fmtHours, dateStr, addDays, showPhoto, GENDER } from '../util.js';
import { segmentRows } from './shared.js';

export default {
  title: '我的紀錄',
  tab: 'records',
  async render(main, ctx) {
    const me = ctx.me;
    let to = dateStr();
    let from = addDays(to, -13);

    main.innerHTML = String(html`
      <div class="stack">
        <section class="card">
          <div class="range-row">
            <label class="field field-inline"><span>從</span><input type="date" name="from" value="${from}"></label>
            <label class="field field-inline"><span>到</span><input type="date" name="to" value="${to}"></label>
          </div>
          <div class="summary-line"></div>
        </section>
        <section class="records"></section>
        <section class="card profile">
          <h2>個人資料</h2>
          <dl class="kv">
            <dt>姓名</dt><dd>${me.name}</dd>
            <dt>員工編號</dt><dd>${me.employeeNo || '—'}</dd>
            <dt>職稱</dt><dd>${me.title || '—'}</dd>
            <dt>電話</dt><dd>${me.phone}</dd>
            <dt>性別</dt><dd>${GENDER[me.gender] || '—'}</dd>
            <dt>到職日</dt><dd>${me.hireDate || '—'}</dd>
          </dl>
          <p class="muted small">個人資料如需修改，請聯絡管理者。</p>
          <a class="btn btn-secondary btn-block" href="#/change-password">更改密碼</a>
        </section>
      </div>`);

    const list = main.querySelector('.records');
    const summary = main.querySelector('.summary-line');

    async function load() {
      list.innerHTML = '<div class="spinner"></div>';
      const sum = await api.get(`/api/me/punches?from=${from}&to=${to}`);
      summary.innerHTML = String(html`
        <div><strong>${fmtHours(sum.totalHours)}</strong><span>總工時</span></div>
        <div><strong>${sum.daysWorked}</strong><span>出勤天數</span></div>
        ${sum.incomplete ? html`<div class="warn"><strong>${sum.incomplete}</strong><span>未打下班卡</span></div>` : ''}`);
      const days = sum.days.filter((d) => d.segments.length).reverse();
      list.innerHTML = days.length
        ? String(html`${days.map((d) => html`
            <article class="card day-card">
              <header><h3>${fmtDateShort(d.date)}</h3><span class="hours">${fmtHours(d.hours)}</span></header>
              ${segmentRows(d.segments)}
            </article>`)}`)
        : '<div class="empty"><p>這段期間沒有打卡紀錄</p></div>';
    }

    main.addEventListener('change', (e) => {
      if (e.target.name === 'from') from = e.target.value || from;
      if (e.target.name === 'to') to = e.target.value || to;
      if (e.target.type === 'date') load().catch(() => {});
    });
    main.addEventListener('click', (e) => {
      const t = e.target.closest('[data-photo]');
      if (t) showPhoto(t.dataset.photo, t.dataset.caption);
    });
    await load();
  },
};


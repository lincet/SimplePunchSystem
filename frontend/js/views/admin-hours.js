import { api } from '../api.js';
import { html, fmtTime, fmtHours, fmtDateLong, fmtDateShort, dateStr, addDays, addMonths } from '../util.js';

const PERIODS = { day: '單日', week: '單週', month: '單月' };

export default {
  title: '工時統計',
  tab: 'hours',
  async render(main, ctx) {
    let period = ctx.query.get('period') || 'day';
    let date = ctx.query.get('date') || dateStr();

    main.innerHTML = String(html`
      <div class="stack">
        <div class="segmented segmented-lg" role="tablist">
          ${Object.entries(PERIODS).map(([k, v]) => html`
            <label><input type="radio" name="period" value="${k}" ${k === period ? 'checked' : ''}><span>${v}</span></label>`)}
        </div>
        <div class="period-nav card">
          <button type="button" class="icon-btn" data-step="-1" aria-label="上一期">‹</button>
          <div class="period-label"></div>
          <button type="button" class="icon-btn" data-step="1" aria-label="下一期">›</button>
        </div>
        <section class="hours-total card"></section>
        <ul class="list hours-list"></ul>
      </div>`);

    const label = main.querySelector('.period-label');
    const total = main.querySelector('.hours-total');
    const list = main.querySelector('.hours-list');

    async function load() {
      list.innerHTML = '<li class="spinner"></li>';
      const data = await api.get(`/api/admin/hours?period=${period}&date=${date}`);
      label.innerHTML = String(period === 'day'
        ? html`<strong>${fmtDateLong(data.from)}</strong>`
        : period === 'week'
          ? html`<strong>${fmtDateShort(data.from)} – ${fmtDateShort(data.to)}</strong>`
          : html`<strong>${data.from.slice(0, 4)} 年 ${Number(data.from.slice(5, 7))} 月</strong>`);
      if (data.to >= dateStr() && data.from <= dateStr()) label.insertAdjacentHTML('beforeend', '<small>（本期）</small>');

      total.innerHTML = String(html`
        <div><span class="muted">全體總工時</span><strong>${fmtHours(data.totalHours)}</strong></div>
        <div><span class="muted">員工數</span><strong>${data.reports.length}</strong></div>`);

      list.innerHTML = data.reports.length
        ? String(html`${data.reports.map((r) => {
            const s = r.summary;
            const days = s.days.filter((d) => d.segments.length);
            return html`
              <li class="card hours-item">
                <details>
                  <summary>
                    <div class="list-main">
                      <strong>${r.user.name}</strong>
                      <span class="muted">${period === 'day' ? '' : `出勤 ${s.daysWorked} 天`}
                        ${s.incomplete ? html`<span class="tag tag-warn">未打下班卡 ${s.incomplete}</span>` : ''}</span>
                    </div>
                    <span class="hours-big">${fmtHours(s.totalHours)}</span>
                  </summary>
                  ${days.length ? html`
                    <table class="hours-table">
                      <thead><tr><th>日期</th><th>時段</th><th>時數</th></tr></thead>
                      <tbody>
                        ${days.map((d) => html`
                          <tr>
                            <td>${fmtDateShort(d.date)}</td>
                            <td>${d.segments.map((sg) => html`<div>${timeRange(sg)}</div>`)}</td>
                            <td>${fmtHours(d.hours)}</td>
                          </tr>`)}
                      </tbody>
                    </table>` : html`<p class="muted small">這段期間沒有打卡</p>`}
                </details>
              </li>`;
          })}`)
        : '<li class="empty"><p>沒有員工資料</p></li>';
    }

    main.addEventListener('change', (e) => {
      if (e.target.name !== 'period') return;
      period = e.target.value;
      load().catch(() => {});
    });
    main.addEventListener('click', (e) => {
      const b = e.target.closest('[data-step]');
      if (!b) return;
      const n = Number(b.dataset.step);
      date = period === 'day' ? addDays(date, n) : period === 'week' ? addDays(date, 7 * n) : addMonths(date, n);
      load().catch(() => {});
    });
    await load();
  },
};

function timeRange(sg) {
  return `${fmtTime(sg.in)}–${sg.out ? fmtTime(sg.out) : sg.open ? '（上班中）' : '（未打卡）'}`;
}

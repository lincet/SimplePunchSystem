import { api } from '../api.js';
import { html, fmtDateLong, fmtHours, dateStr, addDays, showPhoto } from '../util.js';
import { segmentRows, employeeSelect } from './shared.js';

export default {
  title: '出勤紀錄',
  tab: 'attendance',
  async render(main) {
    const users = await api.get('/api/admin/users?all=1');
    let user = '';
    let to = dateStr();
    let from = addDays(to, -6);

    main.innerHTML = String(html`
      <div class="stack">
        <section class="card filters">
          <label class="field"><span>員工</span>${employeeSelect(users, user)}</label>
          <div class="range-row">
            <label class="field field-inline"><span>從</span><input type="date" name="from" value="${from}"></label>
            <label class="field field-inline"><span>到</span><input type="date" name="to" value="${to}"></label>
          </div>
          <div class="chips">
            <button type="button" class="chip" data-range="today">今天</button>
            <button type="button" class="chip" data-range="7">近 7 天</button>
            <button type="button" class="chip" data-range="30">近 30 天</button>
          </div>
        </section>
        <section class="attendance"></section>
      </div>`);

    const out = main.querySelector('.attendance');
    const setRange = (f, t) => {
      from = f; to = t;
      main.querySelector('[name=from]').value = f;
      main.querySelector('[name=to]').value = t;
    };

    async function load() {
      out.innerHTML = '<div class="spinner"></div>';
      const data = await api.get(`/api/admin/attendance?from=${from}&to=${to}${user ? `&user_id=${user}` : ''}`);
      // 依日期（新到舊）分組，每天列出各員工的時段
      const byDate = new Map();
      for (const rep of data.reports) {
        for (const d of rep.summary.days) {
          if (!d.segments.length) continue;
          if (!byDate.has(d.date)) byDate.set(d.date, []);
          byDate.get(d.date).push({ user: rep.user, day: d });
        }
      }
      const dates = [...byDate.keys()].sort().reverse();
      out.innerHTML = dates.length
        ? String(html`${dates.map((date) => html`
            <article class="day-group">
              <h2 class="day-title">${fmtDateLong(date)}</h2>
              ${byDate.get(date).map(({ user: u, day }) => html`
                <div class="card day-card">
                  <header>
                    <h3>${u.name}${u.active ? '' : html` <small class="muted">（已停用）</small>`}</h3>
                    <span class="hours">${fmtHours(day.hours)}</span>
                  </header>
                  ${segmentRows(day.segments)}
                </div>`)}
            </article>`)}`)
        : '<div class="empty"><p>這段期間沒有打卡紀錄</p></div>';
    }

    main.addEventListener('change', (e) => {
      const t = e.target;
      if (t.name === 'user') user = t.value;
      else if (t.name === 'from') from = t.value || from;
      else if (t.name === 'to') to = t.value || to;
      else return;
      load().catch(() => {});
    });
    main.addEventListener('click', (e) => {
      const ph = e.target.closest('[data-photo]');
      if (ph) return showPhoto(ph.dataset.photo, ph.dataset.caption);
      const chip = e.target.closest('[data-range]');
      if (!chip) return;
      const today = dateStr();
      if (chip.dataset.range === 'today') setRange(today, today);
      else setRange(addDays(today, 1 - Number(chip.dataset.range)), today);
      load().catch(() => {});
    });
    await load();
  },
};

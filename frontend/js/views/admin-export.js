import { api } from '../api.js';
import { html, busy, toast, dateStr, monthStart, monthEnd, addMonths } from '../util.js';
import { employeeSelect } from './shared.js';

export default {
  title: '匯出報表',
  tab: 'export',
  async render(main) {
    const users = await api.get('/api/admin/users?all=1');
    const today = dateStr();

    main.innerHTML = String(html`
      <div class="stack">
        <form class="card form" novalidate>
          <div class="chips">
            <button type="button" class="chip" data-preset="this">本月</button>
            <button type="button" class="chip" data-preset="last">上個月</button>
          </div>
          <div class="range-row">
            <label class="field field-inline"><span>從</span><input type="date" name="from" value="${monthStart(today)}"></label>
            <label class="field field-inline"><span>到</span><input type="date" name="to" value="${today}"></label>
          </div>
          <label class="field"><span>員工</span>${employeeSelect(users, '')}</label>
          <p class="form-error" role="alert" hidden></p>
          <button type="button" class="btn btn-primary btn-block btn-lg" data-format="xlsx">下載 Excel（.xlsx）</button>
          <button type="button" class="btn btn-secondary btn-block" data-format="csv">下載 CSV</button>
        </form>
        <div class="card muted small">
          <p><strong>Excel 檔內容</strong></p>
          <ul class="bullets">
            <li>「工時統計」：每位員工的出勤天數、總時數、未打下班卡次數</li>
            <li>「打卡明細」：每一段上下班時間與時數</li>
          </ul>
          <p>CSV 只包含打卡明細，可用 Excel 或 Google 試算表開啟。</p>
        </div>
      </div>`);

    const form = main.querySelector('form');
    const err = form.querySelector('.form-error');
    const from = form.querySelector('[name=from]');
    const to = form.querySelector('[name=to]');

    main.addEventListener('click', (e) => {
      const p = e.target.closest('[data-preset]');
      if (p) {
        const base = p.dataset.preset === 'this' ? today : addMonths(today, -1);
        from.value = monthStart(base);
        to.value = p.dataset.preset === 'this' ? today : monthEnd(base);
        return;
      }
      const b = e.target.closest('[data-format]');
      if (!b) return;
      err.hidden = true;
      if (!from.value || !to.value || from.value > to.value) {
        err.textContent = '請選擇正確的日期區間';
        err.hidden = false;
        return;
      }
      const user = form.querySelector('[name=user]').value;
      const fmt = b.dataset.format;
      busy(b, async () => {
        try {
          await api.download(
            `/api/admin/export?format=${fmt}&from=${from.value}&to=${to.value}${user ? `&user_id=${user}` : ''}`,
            `attendance.${fmt}`);
          toast('已開始下載', 'success');
        } catch (ex) {
          err.textContent = ex.message;
          err.hidden = false;
        }
      });
    });
  },
};

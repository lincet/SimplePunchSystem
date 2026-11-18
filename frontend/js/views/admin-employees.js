import { api } from '../api.js';
import { html, GENDER } from '../util.js';

export default {
  title: '員工管理',
  tab: 'employees',
  async render(main) {
    let showInactive = false;

    main.innerHTML = String(html`
      <div class="stack">
        <div class="toolbar">
          <label class="switch"><input type="checkbox" name="inactive"><span>顯示已停用</span></label>
          <a class="btn btn-primary" href="#/admin/employees/new">＋ 新增員工</a>
        </div>
        <ul class="list employee-list"><li class="spinner"></li></ul>
      </div>`);
    const list = main.querySelector('.employee-list');

    async function load() {
      const users = await api.get(`/api/admin/users${showInactive ? '?all=1' : ''}`);
      list.innerHTML = users.length
        ? String(html`${users.map((u) => html`
            <li>
              <a class="list-item ${u.active ? '' : 'is-inactive'}" href="#/admin/employees/${u.id}">
                <div class="avatar avatar-${u.gender}">${u.name.slice(0, 1)}</div>
                <div class="list-main">
                  <strong>${u.name}</strong>
                  <span class="muted">${[u.employeeNo, u.title, GENDER[u.gender]].filter(Boolean).join('・')}</span>
                  <span class="muted">${u.phone}</span>
                </div>
                <div class="list-badges">
                  ${u.active ? '' : html`<span class="tag tag-muted">已停用</span>`}
                  ${u.active && u.mustChangePassword ? html`<span class="tag tag-warn">待改密碼</span>` : ''}
                </div>
                <span class="chev" aria-hidden="true">›</span>
              </a>
            </li>`)}`)
        : String(html`<li class="empty"><p>還沒有員工資料</p><p class="muted small">點右上角「新增員工」開始建立</p></li>`);
    }

    main.querySelector('[name=inactive]').addEventListener('change', (e) => {
      showInactive = e.target.checked;
      load();
    });
    await load();
  },
};

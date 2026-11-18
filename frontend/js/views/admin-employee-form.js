import { api } from '../api.js';
import { html, busy, toast, randomPassword } from '../util.js';

export default {
  title: (params) => (params.id ? '編輯員工' : '新增員工'),
  back: '#/admin/employees',
  tab: 'employees',
  async render(main, ctx) {
    const isNew = !ctx.params.id;
    const u = isNew
      ? { name: '', phone: '', gender: '', employeeNo: '', title: '', hireDate: '', email: '', note: '', active: true }
      : await api.get(`/api/admin/users/${encodeURIComponent(ctx.params.id)}`);

    const genderRadio = (value, label) => html`
      <label><input type="radio" name="gender" value="${value}" ${u.gender === value ? 'checked' : ''}><span>${label}</span></label>`;

    main.innerHTML = String(html`
      <div class="stack">
        <form class="card form employee-form" novalidate>
          <label class="field">
            <span>姓名 <em>*</em></span>
            <input name="name" value="${u.name}" autocomplete="off" required maxlength="50">
          </label>
          <label class="field">
            <span>手機號碼 <em>*</em></span>
            <input name="phone" value="${u.phone}" type="tel" inputmode="tel" autocomplete="off" required>
            <small class="hint">員工以此號碼作為登入帳號</small>
          </label>
          <fieldset class="field">
            <legend>性別 <em>*</em></legend>
            <div class="segmented">
              ${genderRadio('male', '男')}${genderRadio('female', '女')}${genderRadio('other', '其他')}
            </div>
          </fieldset>
          <div class="grid-2">
            <label class="field"><span>員工編號</span><input name="employeeNo" value="${u.employeeNo}" autocomplete="off"></label>
            <label class="field"><span>職稱</span><input name="title" value="${u.title}" autocomplete="off"></label>
          </div>
          <label class="field"><span>到職日</span><input name="hireDate" type="date" value="${u.hireDate}"></label>
          <label class="field"><span>Email</span><input name="email" type="email" inputmode="email" value="${u.email}" autocomplete="off"></label>
          <label class="field"><span>備註</span><textarea name="note" rows="2" maxlength="500">${u.note}</textarea></label>
          ${isNew ? html`
            <label class="field">
              <span>預設密碼 <em>*</em></span>
              <div class="input-action">
                <input name="password" type="text" autocomplete="off" autocapitalize="off" spellcheck="false" minlength="8" required>
                <button type="button" class="btn btn-secondary" data-gen>產生</button>
              </div>
              <small class="hint">至少 8 個字元。請告知員工，員工第一次登入時必須改成自己的密碼。</small>
            </label>` : html`
            <label class="switch switch-row">
              <span><strong>帳號啟用</strong><small class="muted">停用後員工將無法登入與打卡</small></span>
              <input type="checkbox" name="active" ${u.active ? 'checked' : ''}>
            </label>`}
          <p class="form-error" role="alert" hidden></p>
          <button class="btn btn-primary btn-block btn-lg" type="submit">${isNew ? '建立員工' : '儲存'}</button>
        </form>

        ${isNew ? '' : html`
          <form class="card form reset-form" novalidate>
            <h2>重設密碼</h2>
            <p class="muted small">員工忘記密碼時使用。重設後員工需用新的預設密碼登入，並再次設定自己的密碼。</p>
            <div class="input-action">
              <input name="password" type="text" autocomplete="off" autocapitalize="off" spellcheck="false"
                placeholder="新的預設密碼（至少 8 碼）" minlength="8">
              <button type="button" class="btn btn-secondary" data-gen>產生</button>
            </div>
            <p class="form-error" role="alert" hidden></p>
            <button class="btn btn-danger-outline btn-block" type="submit">重設密碼</button>
          </form>`}
      </div>`);

    main.querySelectorAll('[data-gen]').forEach((b) => b.addEventListener('click', () => {
      b.closest('.input-action').querySelector('input').value = randomPassword(8);
    }));

    const form = main.querySelector('.employee-form');
    const field = (n) => form.querySelector(`[name="${n}"]`);
    const err = form.querySelector('.form-error');
    const fail = (m) => { err.textContent = m; err.hidden = false; err.scrollIntoView({ block: 'center', behavior: 'smooth' }); };

    form.addEventListener('submit', (e) => {
      e.preventDefault();
      err.hidden = true;
      const data = {
        name: field('name').value.trim(),
        phone: field('phone').value.trim(),
        gender: form.querySelector('[name=gender]:checked')?.value || '',
        employeeNo: field('employeeNo').value.trim(),
        title: field('title').value.trim(),
        hireDate: field('hireDate').value,
        email: field('email').value.trim(),
        note: field('note').value.trim(),
      };
      if (!data.name) return fail('請輸入姓名');
      if (!data.phone) return fail('請輸入手機號碼');
      if (!data.gender) return fail('請選擇性別');
      if (isNew) {
        data.password = field('password').value;
        if (data.password.length < 8) return fail('預設密碼至少需要 8 個字元');
      } else {
        data.active = field('active').checked;
        if (u.active && !data.active && !confirm(`確定要停用「${u.name}」嗎？停用後將無法登入與打卡。`)) return;
      }
      busy(form.querySelector('button[type=submit]'), async () => {
        try {
          if (isNew) {
            await api.post('/api/admin/users', data);
            toast(`已建立 ${data.name}，帳號 ${data.phone.replace(/[\s-]/g, '')}`, 'success');
          } else {
            await api.put(`/api/admin/users/${u.id}`, data);
            toast('已儲存', 'success');
          }
          ctx.go('#/admin/employees');
        } catch (ex) {
          fail(ex.message);
        }
      });
    });

    const reset = main.querySelector('.reset-form');
    reset?.addEventListener('submit', (e) => {
      e.preventDefault();
      const rerr = reset.querySelector('.form-error');
      rerr.hidden = true;
      const pw = reset.password.value;
      if (pw.length < 8) {
        rerr.textContent = '預設密碼至少需要 8 個字元';
        rerr.hidden = false;
        return;
      }
      if (!confirm(`確定要把「${u.name}」的密碼重設為「${pw}」嗎？`)) return;
      busy(reset.querySelector('button[type=submit]'), async () => {
        try {
          await api.post(`/api/admin/users/${u.id}/reset-password`, { password: pw });
          toast('密碼已重設，請告知員工新的預設密碼', 'success');
          reset.password.value = '';
        } catch (ex) {
          rerr.textContent = ex.message;
          rerr.hidden = false;
        }
      });
    });
  },
};

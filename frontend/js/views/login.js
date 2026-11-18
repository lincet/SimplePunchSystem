import { api } from '../api.js';
import { html, busy } from '../util.js';

export default {
  title: '登入',
  bare: true,
  render(main, ctx) {
    const demo = ctx.health?.demo ? ctx.health.demoAccounts || [] : [];
    main.innerHTML = String(html`
      <div class="login">
        <div class="login-brand">
          <div class="login-logo" aria-hidden="true">
            <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>
          </div>
          <h1>上班打卡</h1>
          <p>請使用管理者提供的帳號登入</p>
        </div>
        <form class="card form" novalidate>
          <label class="field">
            <span>帳號（手機號碼）</span>
            <input name="username" type="text" inputmode="tel" autocomplete="username" autocapitalize="off"
              autocorrect="off" spellcheck="false" required>
          </label>
          <label class="field">
            <span>密碼</span>
            <div class="password-wrap">
              <input name="password" type="password" autocomplete="current-password" required>
              <button type="button" class="password-toggle" aria-label="顯示密碼">顯示</button>
            </div>
          </label>
          <p class="form-error" role="alert" hidden></p>
          <button class="btn btn-primary btn-block btn-lg" type="submit">登入</button>
        </form>
        ${demo.length ? html`
          <section class="card demo-box">
            <h2>示範帳號</h2>
            <p class="muted small">點一下即可帶入帳號密碼</p>
            <ul>
              ${demo.map((a) => html`
                <li><button type="button" class="demo-account" data-u="${a.username}" data-p="${a.password}">
                  <strong>${a.label}</strong><span>${a.username} / ${a.password}</span>
                </button></li>`)}
            </ul>
          </section>` : ''}
      </div>`);

    const form = main.querySelector('form');
    const err = main.querySelector('.form-error');
    const pw = form.password;
    main.querySelector('.password-toggle').addEventListener('click', (e) => {
      pw.type = pw.type === 'password' ? 'text' : 'password';
      e.currentTarget.textContent = pw.type === 'password' ? '顯示' : '隱藏';
    });
    main.querySelectorAll('.demo-account').forEach((b) => b.addEventListener('click', () => {
      form.username.value = b.dataset.u;
      pw.value = b.dataset.p.startsWith('（') ? '' : b.dataset.p;
      (pw.value ? form.querySelector('button[type=submit]') : pw).focus();
    }));

    form.addEventListener('submit', (e) => {
      e.preventDefault();
      err.hidden = true;
      const username = form.username.value.trim();
      if (!username || !pw.value) {
        err.textContent = '請輸入帳號與密碼';
        err.hidden = false;
        return;
      }
      busy(form.querySelector('button[type=submit]'), async () => {
        try {
          const me = await api.post('/api/auth/login', { username, password: pw.value });
          ctx.setMe(me);
          ctx.go(ctx.home());
        } catch (ex) {
          err.textContent = ex.message;
          err.hidden = false;
        }
      });
    });
  },
};

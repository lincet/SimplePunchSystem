import { api } from '../api.js';
import { html, busy, toast } from '../util.js';

export default {
  title: '更改密碼',
  render(main, ctx) {
    const forced = ctx.me.mustChangePassword;
    main.innerHTML = String(html`
      <div class="stack">
        ${forced ? html`
          <div class="notice notice-warn">
            <strong>第一次登入，請設定你自己的密碼</strong>
            <p>目前使用的是管理者給的預設密碼，更改後才能開始使用。</p>
          </div>` : ''}
        <form class="card form" novalidate>
          <label class="field">
            <span>${forced ? '預設密碼' : '目前密碼'}</span>
            <input name="old" type="password" autocomplete="current-password" required>
          </label>
          <label class="field">
            <span>新密碼</span>
            <input name="new1" type="password" autocomplete="new-password" minlength="8" required>
            <small class="hint">至少 8 個字元</small>
          </label>
          <label class="field">
            <span>再輸入一次新密碼</span>
            <input name="new2" type="password" autocomplete="new-password" minlength="8" required>
          </label>
          <p class="form-error" role="alert" hidden></p>
          <button class="btn btn-primary btn-block btn-lg" type="submit">確認更改</button>
          ${forced ? '' : html`<a class="btn btn-ghost btn-block" href="${ctx.home()}">取消</a>`}
        </form>
      </div>`);

    const form = main.querySelector('form');
    const err = main.querySelector('.form-error');
    const fail = (m) => { err.textContent = m; err.hidden = false; };

    form.addEventListener('submit', (e) => {
      e.preventDefault();
      err.hidden = true;
      const { old, new1, new2 } = form;
      if (!old.value) return fail('請輸入目前的密碼');
      if (new1.value.length < 8) return fail('新密碼至少需要 8 個字元');
      if (new1.value !== new2.value) return fail('兩次輸入的新密碼不一樣');
      busy(form.querySelector('button[type=submit]'), async () => {
        try {
          const me = await api.post('/api/me/password', { oldPassword: old.value, newPassword: new1.value });
          ctx.setMe(me);
          toast('密碼已更新', 'success');
          ctx.go(ctx.home());
        } catch (ex) {
          fail(ex.message);
        }
      });
    });
  },
};

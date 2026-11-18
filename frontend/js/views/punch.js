import { api } from '../api.js';
import { capturePhoto } from '../camera.js';
import { html, fmtTime, fmtDateLong, fmtDuration, showPhoto, toast } from '../util.js';

const LABEL = { in: '上班打卡', out: '下班打卡' };

export default {
  title: '打卡',
  tab: 'punch',
  async render(main, ctx) {
    let status = await api.get('/api/punch/status');
    // 用伺服器時間校正手機時鐘
    let offset = new Date(status.serverTime).getTime() - Date.now();
    const now = () => new Date(Date.now() + offset);

    main.innerHTML = String(html`
      <div class="punch">
        <p class="greeting">${greeting(now())}，${ctx.me.name}</p>
        <div class="clock" aria-live="off">
          <div class="clock-time"></div>
          <div class="clock-date"></div>
        </div>
        <div class="status-card"></div>
        <button class="punch-btn" type="button"></button>
        <p class="punch-hint">打卡時需要拍一張照片</p>
        <section class="today">
          <h2>今日打卡</h2>
          <ul class="today-list"></ul>
        </section>
      </div>`);

    const q = (s) => main.querySelector(s);
    const btn = q('.punch-btn');

    function paintClock() {
      const n = now();
      q('.clock-time').textContent = fmtTime(n, true);
      q('.clock-date').textContent = fmtDateLong(n);
      if (status.next === 'out' && status.last) {
        const el = q('.status-elapsed');
        if (el) el.textContent = fmtDuration(n - new Date(status.last.punchedAt));
      }
    }

    function paint() {
      const working = status.next === 'out';
      q('.status-card').className = `status-card ${working ? 'is-working' : 'is-off'}`;
      q('.status-card').innerHTML = String(working
        ? html`<span class="dot"></span><div><strong>上班中</strong>
            <p>${fmtTime(status.last.punchedAt)} 開始・已工作 <span class="status-elapsed"></span></p></div>`
        : html`<span class="dot"></span><div><strong>${status.last ? '已下班' : '尚未打卡'}</strong>
            <p>${status.last?.type === 'out' ? `上次下班 ${fmtTime(status.last.punchedAt)}` : '準備好就可以打上班卡'}</p></div>`);
      btn.className = `punch-btn punch-btn-${status.next}`;
      btn.innerHTML = String(html`
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 8h3l2-3h6l2 3h3v11H4z"/><circle cx="12" cy="13" r="3.5"/></svg>
        <span>${LABEL[status.next]}</span>`);

      const list = q('.today-list');
      list.innerHTML = status.today.length
        ? String(html`${[...status.today].reverse().map((p) => html`
            <li class="punch-row">
              <button type="button" class="thumb" data-photo="${p.photoUrl}" data-caption="${LABEL[p.type]} ${fmtTime(p.punchedAt)}">
                <img src="${p.photoUrl}" alt="" loading="lazy">
              </button>
              <div><span class="tag tag-${p.type}">${p.type === 'in' ? '上班' : '下班'}</span></div>
              <time>${fmtTime(p.punchedAt)}</time>
            </li>`)}`)
        : '<li class="muted empty-row">今天還沒有打卡紀錄</li>';
      paintClock();
    }

    main.addEventListener('click', (e) => {
      const t = e.target.closest('[data-photo]');
      if (t) showPhoto(t.dataset.photo, t.dataset.caption);
    });

    btn.addEventListener('click', async () => {
      const type = status.next;
      const blob = await capturePhoto({ title: LABEL[type], accent: type, now });
      if (!blob) return;
      btn.disabled = true;
      btn.classList.add('is-busy');
      btn.querySelector('span').textContent = '上傳中…';
      try {
        const form = new FormData();
        form.append('type', type);
        form.append('photo', blob, 'punch.jpg');
        status = await api.upload('/api/punch', form);
        offset = new Date(status.serverTime).getTime() - Date.now();
        success(type, status.punch.punchedAt);
      } catch (ex) {
        toast(ex.message, 'error');
        try { status = await api.get('/api/punch/status'); } catch { /* 保留原狀態 */ }
      } finally {
        btn.disabled = false;
        btn.classList.remove('is-busy');
        paint();
      }
    });

    paint();
    const timer = setInterval(paintClock, 1000);

    // 從背景回來時（例如隔天打開）重新整理狀態
    const onVisible = async () => {
      if (document.visibilityState !== 'visible') return;
      try {
        status = await api.get('/api/punch/status');
        offset = new Date(status.serverTime).getTime() - Date.now();
        paint();
      } catch { /* 忽略 */ }
    };
    document.addEventListener('visibilitychange', onVisible);

    return () => {
      clearInterval(timer);
      document.removeEventListener('visibilitychange', onVisible);
    };
  },
};

function greeting(d) {
  const h = Number(fmtTime(d).slice(0, 2));
  if (h < 5) return '晚安';
  if (h < 11) return '早安';
  if (h < 18) return '午安';
  return '晚安';
}

function success(type, at) {
  const el = document.createElement('div');
  el.className = `success-overlay success-${type}`;
  el.innerHTML = String(html`
    <div class="success-card">
      <div class="success-check">✓</div>
      <h2>${LABEL[type]}成功</h2>
      <p class="success-time">${fmtTime(at)}</p>
      <p class="muted">${fmtDateLong(at)}</p>
    </div>`);
  document.body.appendChild(el);
  const close = () => el.remove();
  el.addEventListener('click', close);
  setTimeout(close, 2200);
}

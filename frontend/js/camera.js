// 打卡拍照：全螢幕相機 → 拍照 → 預覽／重拍 → 確認。
// 優先用 getUserMedia（iOS Safari 14.3+、Android Chrome 皆支援，需 HTTPS）；
// 失敗時退回 <input capture>，直接開啟手機系統相機。

import { html, fmtTime, dateStr } from './util.js';

const MAX_SIDE = 960;
const QUALITY = 0.75;

/**
 * 開啟相機並回傳 JPEG Blob；使用者取消時回傳 null。
 * @param {{title: string, accent: string, now: () => Date}} opts
 */
export function capturePhoto({ title, accent = 'in', now = () => new Date() }) {
  return new Promise((resolve) => {
    const el = document.createElement('div');
    el.className = `camera camera-${accent}`;
    el.innerHTML = String(html`
      <div class="camera-top">
        <button class="camera-cancel" type="button">取消</button>
        <div class="camera-title">${title}</div>
        <button class="camera-switch" type="button" aria-label="切換鏡頭" hidden>⟲</button>
      </div>
      <div class="camera-stage">
        <video playsinline autoplay muted></video>
        <img class="camera-preview" alt="照片預覽" hidden>
        <div class="camera-guide" aria-hidden="true"></div>
        <div class="camera-msg" hidden></div>
      </div>
      <div class="camera-bottom">
        <div class="camera-shoot-row">
          <button class="camera-shutter" type="button" aria-label="拍照" disabled></button>
        </div>
        <div class="camera-confirm-row" hidden>
          <button class="btn btn-ghost-light camera-retake" type="button">重拍</button>
          <button class="btn btn-${accent} camera-use" type="button">使用這張照片</button>
        </div>
        <button class="btn btn-${accent} camera-native" type="button" hidden>開啟手機相機</button>
      </div>
      <input type="file" accept="image/*" capture="user" hidden>`);
    document.body.appendChild(el);
    document.body.classList.add('no-scroll');

    const q = (s) => el.querySelector(s);
    const video = q('video');
    const preview = q('.camera-preview');
    const msg = q('.camera-msg');
    const shutter = q('.camera-shutter');
    const fileInput = q('input[type=file]');
    let stream = null;
    let facing = 'user';
    let blob = null;
    let previewURL = null;

    const stop = () => {
      stream?.getTracks().forEach((t) => t.stop());
      stream = null;
    };
    const finish = (result) => {
      stop();
      if (previewURL) URL.revokeObjectURL(previewURL);
      document.body.classList.remove('no-scroll');
      el.remove();
      resolve(result);
    };
    const showMsg = (text) => { msg.textContent = text; msg.hidden = !text; };

    const showPreview = (b) => {
      blob = b;
      if (previewURL) URL.revokeObjectURL(previewURL);
      previewURL = URL.createObjectURL(b);
      preview.src = previewURL;
      preview.hidden = false;
      video.hidden = true;
      q('.camera-guide').hidden = true;
      q('.camera-shoot-row').hidden = true;
      q('.camera-native').hidden = true;
      q('.camera-confirm-row').hidden = false;
      showMsg('');
    };

    const useNativeFallback = (reason) => {
      stop();
      video.hidden = true;
      q('.camera-shoot-row').hidden = true;
      q('.camera-native').hidden = false;
      showMsg(reason);
    };

    async function start() {
      stop();
      if (!navigator.mediaDevices?.getUserMedia) {
        useNativeFallback('此瀏覽器無法直接開啟相機，請點下方按鈕使用手機相機拍照。');
        return;
      }
      showMsg('正在開啟相機…');
      try {
        stream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode: facing, width: { ideal: 1280 }, height: { ideal: 1280 } },
          audio: false,
        });
        video.srcObject = stream;
        video.classList.toggle('mirror', facing === 'user');
        await video.play().catch(() => {});
        showMsg('');
        shutter.disabled = false;
        // 有多顆鏡頭才顯示切換鈕
        const devices = await navigator.mediaDevices.enumerateDevices().catch(() => []);
        q('.camera-switch').hidden = devices.filter((d) => d.kind === 'videoinput').length < 2;
      } catch (e) {
        const denied = e && (e.name === 'NotAllowedError' || e.name === 'SecurityError');
        useNativeFallback(denied
          ? '相機權限被拒絕。請到手機「設定」允許此網站使用相機，或點下方按鈕用手機相機拍照。'
          : '無法開啟相機，請點下方按鈕使用手機相機拍照。');
      }
    }

    shutter.addEventListener('click', async () => {
      if (!video.videoWidth) return;
      shutter.disabled = true;
      const b = await drawToJpeg(video, video.videoWidth, video.videoHeight, facing === 'user', now());
      stop();
      showPreview(b);
    });

    q('.camera-retake').addEventListener('click', () => {
      blob = null;
      preview.hidden = true;
      video.hidden = false;
      q('.camera-guide').hidden = false;
      q('.camera-confirm-row').hidden = true;
      q('.camera-shoot-row').hidden = false;
      shutter.disabled = true;
      if (q('.camera-native').dataset.used) {
        useNativeFallback('');
      } else {
        start();
      }
    });

    q('.camera-use').addEventListener('click', () => finish(blob));
    q('.camera-cancel').addEventListener('click', () => finish(null));
    q('.camera-switch').addEventListener('click', () => {
      facing = facing === 'user' ? 'environment' : 'user';
      start();
    });

    q('.camera-native').addEventListener('click', () => {
      q('.camera-native').dataset.used = '1';
      fileInput.value = '';
      fileInput.click();
    });
    fileInput.addEventListener('change', async () => {
      const file = fileInput.files?.[0];
      if (!file) return;
      try {
        const img = await loadImage(file);
        showPreview(await drawToJpeg(img, img.naturalWidth || img.width, img.naturalHeight || img.height, false, now()));
      } catch {
        showMsg('照片讀取失敗，請再試一次。');
      }
    });

    start();
  });
}

function loadImage(file) {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => { URL.revokeObjectURL(url); resolve(img); };
    img.onerror = (e) => { URL.revokeObjectURL(url); reject(e); };
    img.src = url;
  });
}

/** 縮圖、鏡像（前鏡頭）、加上時間浮水印，輸出 JPEG。 */
function drawToJpeg(source, sw, sh, mirror, when) {
  const scale = Math.min(1, MAX_SIDE / Math.max(sw, sh));
  const w = Math.round(sw * scale);
  const h = Math.round(sh * scale);
  const canvas = document.createElement('canvas');
  canvas.width = w;
  canvas.height = h;
  const ctx = canvas.getContext('2d');
  ctx.save();
  if (mirror) {
    ctx.translate(w, 0);
    ctx.scale(-1, 1);
  }
  ctx.drawImage(source, 0, 0, w, h);
  ctx.restore();

  const stamp = `${dateStr(when)} ${fmtTime(when, true)}`;
  const fs = Math.max(14, Math.round(w / 26));
  ctx.font = `600 ${fs}px -apple-system, system-ui, sans-serif`;
  const pad = Math.round(fs * 0.5);
  const tw = ctx.measureText(stamp).width;
  ctx.fillStyle = 'rgba(0,0,0,.55)';
  ctx.fillRect(w - tw - pad * 3, h - fs - pad * 3, tw + pad * 2, fs + pad * 2);
  ctx.fillStyle = '#fff';
  ctx.textBaseline = 'top';
  ctx.fillText(stamp, w - tw - pad * 2, h - fs - pad * 2);

  return new Promise((resolve) => canvas.toBlob((b) => resolve(b), 'image/jpeg', QUALITY));
}

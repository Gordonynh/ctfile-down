'use strict';

const VERSION = '0.2.0';
const POLL_MS = 900;

const $ = (s) => document.querySelector(s);

const el = {
  url: $('#url'),
  passcode: $('#passcode'),
  resolveBtn: $('#resolveBtn'),
  notice: $('#notice'),
  result: $('#result'),
  resName: $('#resName'),
  resSize: $('#resSize'),
  downloadBtn: $('#downloadBtn'),
  tasks: $('#tasks'),
  empty: $('#empty'),
  taskCount: $('#taskCount'),
  clearDoneBtn: $('#clearDoneBtn'),
  saveDirText: $('#saveDirText'),
  saveDirChip: $('#saveDirChip'),
  statusText: $('#statusText'),
  toasts: $('#toasts'),
  version: $('#versionText'),
};

let resolved = null;

/* ---------- 格式化 ---------- */

function fmtBytes(n) {
  if (n === null || n === undefined || isNaN(n)) return '—';
  if (n < 1024) return n + ' B';
  const u = ['KB', 'MB', 'GB', 'TB'];
  let v = n, i = -1;
  do { v /= 1024; i++; } while (v >= 1024 && i < u.length - 1);
  return (v >= 100 ? v.toFixed(0) : v.toFixed(1)) + ' ' + u[i];
}

function fmtEta(sec) {
  if (!isFinite(sec) || sec <= 0) return '';
  if (sec < 60) return '剩余 ' + Math.ceil(sec) + ' 秒';
  if (sec < 3600) return '剩余 ' + Math.floor(sec / 60) + ' 分';
  return '剩余 ' + Math.floor(sec / 3600) + ' 小时';
}

function esc(s) {
  return String(s === null || s === undefined ? '' : s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

function notice(text, kind) {
  if (!text) { el.notice.hidden = true; return; }
  el.notice.hidden = false;
  el.notice.className = 'note' + (kind ? ' ' + kind : '');
  el.notice.textContent = text;
}

function toast(text, kind) {
  const n = document.createElement('div');
  n.className = 'toast' + (kind ? ' ' + kind : '');
  n.textContent = text;
  el.toasts.appendChild(n);
  setTimeout(() => {
    n.classList.add('out');
    setTimeout(() => n.remove(), 220);
  }, 3000);
}

function setStatus(text) { el.statusText.textContent = text; }

/* ---------- 链接处理 ---------- */

function normalizeURL(raw) {
  raw = (raw || '').trim();
  if (!raw) return '';
  if (!/^https?:\/\//i.test(raw)) raw = 'https://' + raw;
  return raw;
}

function extractPasscode(raw) {
  try {
    const u = new URL(normalizeURL(raw));
    return u.searchParams.get('p') || u.searchParams.get('password') ||
           u.searchParams.get('passcode') || '';
  } catch (e) {
    const m = raw.match(/[?&](?:p|password|passcode)=([^&#]+)/i);
    return m ? decodeURIComponent(m[1]) : '';
  }
}

/* ---------- API ---------- */

async function api(path, options) {
  const res = await fetch(path, options);
  let data = null;
  try { data = await res.json(); } catch (e) { /* 空响应 */ }
  return { ok: res.ok, data: data || {} };
}

function postJSON(path, body, method) {
  return api(path, {
    method: method || 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

/* ---------- 解析 / 下载 ---------- */

async function doResolve() {
  const url = normalizeURL(el.url.value);
  if (!url) { notice('请先粘贴分享链接。', 'warn'); el.url.focus(); return; }
  if (!el.passcode.value) {
    const p = extractPasscode(url);
    if (p) el.passcode.value = p;
  }

  el.resolveBtn.disabled = true;
  el.result.hidden = true;
  notice('正在解析…');

  try {
    const { ok, data } = await postJSON('/api/resolve', {
      url, passcode: el.passcode.value.trim(),
    });
    if (!ok) {
      el.result.hidden = true;
      if (data.code === 'need_passcode') {
        notice(data.error || '该链接需要提取码。', 'warn');
        el.passcode.classList.add('needed');
        el.passcode.focus();
        el.passcode.select();
      } else {
        notice(data.error || '解析失败。', 'error');
      }
      return;
    }
    el.passcode.classList.remove('needed');
    resolved = { url, passcode: el.passcode.value.trim() };
    el.resName.textContent = data.file_name || '未命名文件';
    el.resSize.textContent = data.size_display || fmtBytes(data.file_size);
    el.result.hidden = false;
    notice('');
  } catch (e) {
    notice('网络错误：' + e.message, 'error');
  } finally {
    el.resolveBtn.disabled = false;
  }
}

async function doDownload() {
  if (!resolved) return;
  el.downloadBtn.disabled = true;
  try {
    const { ok, data } = await postJSON('/api/download', {
      url: resolved.url, passcode: resolved.passcode,
    });
    if (!ok) {
      if (data.code === 'need_passcode') {
        notice(data.error || '该链接需要提取码。', 'warn');
        el.passcode.classList.add('needed');
        el.passcode.focus();
      } else {
        notice(data.error || '无法创建下载任务。', 'error');
      }
      return;
    }
    el.result.hidden = true;
    el.url.value = '';
    el.passcode.value = '';
    el.passcode.classList.remove('needed');
    resolved = null;
    notice('');
    setStatus('已加入队列');
    lastSnapshot = '';
    refreshTasks();
  } catch (e) {
    notice('网络错误：' + e.message, 'error');
  } finally {
    el.downloadBtn.disabled = false;
  }
}

/* ---------- 任务列表 ---------- */

const STATUS_TEXT = {
  resolving: '解析中',
  downloading: '下载中',
  done: '已完成',
  error: '失败',
  canceled: '已取消',
};

function renderItem(t) {
  const pct = Math.max(0, Math.min(100, t.progress || 0));
  const active = t.status === 'downloading' || t.status === 'resolving';

  const bits = [];
  if (t.total > 0 && t.status !== 'resolving') {
    bits.push(fmtBytes(t.downloaded) + ' / ' + fmtBytes(t.total));
    bits.push(pct.toFixed(1) + '%');
  }
  if (active && t.speed > 0) {
    bits.push(fmtBytes(t.speed) + '/s');
    if (t.total > t.downloaded) {
      const eta = fmtEta((t.total - t.downloaded) / t.speed);
      if (eta) bits.push(eta);
    }
  }
  if (!bits.length) bits.push(STATUS_TEXT[t.status] || t.status);

  let acts = '';
  if (active) {
    acts += `<button class="item-act" data-act="cancel" data-id="${t.id}">取消</button>`;
  } else if (t.status === 'error' || t.status === 'canceled') {
    acts += `<button class="item-act" data-act="retry" data-id="${t.id}">重试</button>`;
  }
  acts += `<button class="item-act danger" data-act="remove" data-id="${t.id}">移除</button>`;

  const path = (t.status === 'done' && t.save_path)
    ? `<div class="item-path" title="${esc(t.save_path)}">${esc(t.save_path)}</div>` : '';
  const err = t.error ? `<div class="item-error">${esc(t.error)}</div>` : '';

  return `<div class="item ${t.status}">
    <div class="item-line">
      <span class="dot ${t.status}"></span>
      <span class="item-name" title="${esc(t.file_name || t.url)}">${esc(t.file_name || t.url || t.id)}</span>
      <span class="item-meta">${esc(bits.join('  ·  '))}</span>
      ${acts}
    </div>
    <div class="bar"><i style="width:${pct.toFixed(1)}%"></i></div>
    ${path}${err}
  </div>`;
}

let lastSnapshot = '';

async function refreshTasks() {
  try {
    const { ok, data } = await api('/api/tasks');
    if (!ok || !Array.isArray(data)) return;
    const snap = JSON.stringify(data);
    if (snap === lastSnapshot) return;
    lastSnapshot = snap;

    el.taskCount.textContent = data.length;
    el.empty.hidden = data.length > 0;
    el.tasks.innerHTML = data.map(renderItem).join('');

    const n = { active: 0, done: 0, failed: 0 };
    for (const t of data) {
      if (t.status === 'downloading' || t.status === 'resolving') n.active++;
      else if (t.status === 'done') n.done++;
      else if (t.status === 'error') n.failed++;
    }
    if (n.active) setStatus(`${n.active} 个下载中`);
    else if (n.failed) setStatus(`${n.failed} 个失败`);
    else if (n.done) setStatus(`完成 ${n.done} 个`);
    else setStatus('就绪');

    el.clearDoneBtn.hidden = !data.some((t) => ['done', 'error', 'canceled'].includes(t.status));
  } catch (e) { /* 忽略瞬时错误 */ }
}

async function taskAction(id, act) {
  try {
    if (act === 'cancel') {
      await api('/api/tasks/' + id, { method: 'POST' });
    } else if (act === 'remove') {
      await api('/api/tasks/' + id, { method: 'DELETE' });
    } else if (act === 'retry') {
      const cur = (await api('/api/tasks/' + id)).data;
      const { ok, data } = await postJSON('/api/download', { url: cur.url });
      toast(ok ? '已重新加入队列' : (data.error || '重试失败'), ok ? 'success' : 'error');
    }
  } catch (e) {
    toast('操作失败：' + e.message, 'error');
  }
  lastSnapshot = '';
  refreshTasks();
}

async function clearFinished() {
  const { data } = await api('/api/tasks');
  if (!Array.isArray(data)) return;
  for (const t of data) {
    if (['done', 'error', 'canceled'].includes(t.status)) {
      await api('/api/tasks/' + t.id, { method: 'DELETE' });
    }
  }
  lastSnapshot = '';
  refreshTasks();
}

/* ---------- 事件 ---------- */

el.resolveBtn.addEventListener('click', doResolve);
el.downloadBtn.addEventListener('click', doDownload);
el.url.addEventListener('keydown', (e) => { if (e.key === 'Enter') doResolve(); });
el.passcode.addEventListener('keydown', (e) => { if (e.key === 'Enter') doResolve(); });

el.url.addEventListener('input', () => {
  if (el.passcode.value) return;
  const p = extractPasscode(el.url.value);
  if (p) el.passcode.value = p;
});

el.tasks.addEventListener('click', (e) => {
  const b = e.target.closest('[data-act]');
  if (b) taskAction(b.dataset.id, b.dataset.act);
});

el.clearDoneBtn.addEventListener('click', clearFinished);

el.saveDirChip.addEventListener('click', async () => {
  const text = el.saveDirText.textContent;
  try {
    await navigator.clipboard.writeText(text);
    toast('保存目录已复制', 'success');
  } catch (e) {
    toast(text);
  }
});

/* ---------- 初始化 ---------- */

el.version.textContent = 'v' + VERSION;

(async () => {
  try {
    const { data } = await api('/api/config');
    if (data.save_dir) el.saveDirText.textContent = data.save_dir;
  } catch (e) { /* 忽略 */ }
  refreshTasks();
  setInterval(refreshTasks, POLL_MS);
})();

'use strict';

const $ = (sel) => document.querySelector(sel);

const urlInput = $('#url');
const fileInfo = $('#fileInfo');
const fileName = $('#fileName');
const fileSize = $('#fileSize');
const msg = $('#msg');
const tasksEl = $('#tasks');
const emptyEl = $('#empty');

let currentURL = '';

function showMsg(text, isError) {
  msg.textContent = text;
  msg.classList.remove('hidden', 'error');
  if (isError) msg.classList.add('error');
}

function clearMsg() {
  msg.classList.add('hidden');
  msg.textContent = '';
}

function fmtBytes(n) {
  if (n == null || isNaN(n)) return '—';
  const unit = 1024;
  if (n < unit) return n + ' B';
  const units = ['KB', 'MB', 'GB', 'TB'];
  let v = n;
  let i = -1;
  do { v /= unit; i++; } while (v >= unit && i < units.length - 1);
  return v.toFixed(1) + ' ' + units[i];
}

async function resolveURL() {
  const url = urlInput.value.trim();
  if (!url) { showMsg('请先粘贴链接', true); return; }
  clearMsg();
  fileInfo.classList.add('hidden');
  const btn = $('#resolve');
  btn.disabled = true;
  try {
    const res = await fetch('/api/resolve', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || '解析失败');
    currentURL = url;
    fileName.textContent = data.file_name;
    fileSize.textContent = fmtBytes(data.file_size) + (data.size_display ? '  (' + data.size_display + ')' : '');
    fileInfo.classList.remove('hidden');
  } catch (e) {
    showMsg(e.message, true);
  } finally {
    btn.disabled = false;
  }
}

async function startDownload() {
  if (!currentURL) return;
  const btn = $('#download');
  btn.disabled = true;
  clearMsg();
  try {
    const res = await fetch('/api/download', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url: currentURL }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || '创建下载失败');
    fileInfo.classList.add('hidden');
    urlInput.value = '';
    currentURL = '';
    refreshTasks();
  } catch (e) {
    showMsg(e.message, true);
  } finally {
    btn.disabled = false;
  }
}

function taskStatusBadge(status) {
  const map = {
    resolving: '解析中',
    downloading: '下载中',
    done: '完成',
    error: '失败',
    canceled: '已取消',
  };
  return '<span class="badge ' + status + '">' + (map[status] || status) + '</span>';
}

function renderTask(t) {
  const pct = Math.min(100, Math.max(0, t.progress || 0)).toFixed(1);
  const speed = t.speed > 0 ? fmtBytes(t.speed) + '/s' : '—';
  let foot =
    '<div class="task-foot">' +
    '<span>' + pct + '%</span>' +
    '<span>' + fmtBytes(t.downloaded) + ' / ' + fmtBytes(t.total) + '</span>' +
    '<span>' + speed + '</span>' +
    '<span class="spacer"></span>';
  if (t.status === 'downloading' || t.status === 'resolving') {
    foot += '<button class="link-btn" data-act="cancel" data-id="' + t.id + '">取消</button>';
  }
  foot += '<button class="link-btn" data-act="remove" data-id="' + t.id + '">删除</button>';
  foot += '</div>';

  const err = t.error ? '<div class="task-error">' + escapeHtml(t.error) + '</div>' : '';

  return (
    '<div class="task ' + t.status + '" data-id="' + t.id + '">' +
    '<div class="task-head">' +
    '<span class="task-name">' + escapeHtml(t.file_name || t.id) + '</span>' +
    taskStatusBadge(t.status) +
    '</div>' +
    '<div class="progress-track"><div class="progress-fill" style="width:' + pct + '%"></div></div>' +
    foot + err +
    '</div>'
  );
}

function escapeHtml(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

async function refreshTasks() {
  try {
    const res = await fetch('/api/tasks');
    const tasks = await res.json();
    if (!Array.isArray(tasks)) return;
    emptyEl.classList.toggle('hidden', tasks.length > 0);
    tasksEl.innerHTML = tasks.map(renderTask).join('');
  } catch (e) {
    // ignore transient errors
  }
}

async function taskAction(id, act) {
  try {
    if (act === 'cancel') {
      await fetch('/api/tasks/' + id, { method: 'POST' });
    } else if (act === 'remove') {
      await fetch('/api/tasks/' + id, { method: 'DELETE' });
    }
  } catch (e) {
    // ignore
  }
  refreshTasks();
}

document.addEventListener('click', (e) => {
  const btn = e.target.closest('[data-act]');
  if (!btn) return;
  taskAction(btn.dataset.id, btn.dataset.act);
});

$('#resolve').addEventListener('click', resolveURL);
$('#download').addEventListener('click', startDownload);
urlInput.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') resolveURL();
});

// Poll task progress.
refreshTasks();
setInterval(refreshTasks, 1000);

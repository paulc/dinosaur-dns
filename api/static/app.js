import { RPC } from './rpc.js';
import { Buffer } from './buffer.js';

const rpc = new RPC('/api');
const logBuf = new Buffer(1000);

// ── Stats ──────────────────────────────────────────────────────────────────────

const stats = { total: 0, blocked: 0, aclBlocked: 0, errors: 0, recent: [] };

function qpm() {
    const cutoff = Date.now() - 60000;
    stats.recent = stats.recent.filter(t => t > cutoff);
    return stats.recent.length;
}

function renderStats() {
    const pct = n => stats.total ? ` (${(n * 100 / stats.total).toFixed(1)}%)` : '';
    setText('stat-total',   stats.total);
    setText('stat-blocked', `${stats.blocked}${pct(stats.blocked)}`);
    setText('stat-acl',     `${stats.aclBlocked}${pct(stats.aclBlocked)}`);
    setText('stat-errors',  `${stats.errors}${pct(stats.errors)}`);
    setText('stat-rate',    `${qpm()}/min`);
}

// ── Connection ─────────────────────────────────────────────────────────────────

function setConnected(ok) {
    const el = document.getElementById('indicator');
    el.className = 'indicator ' + (ok ? 'on' : 'off');
    el.title = ok ? 'SSE connected' : 'SSE disconnected';
}

// ── SSE ───────────────────────────────────────────────────────────────────────

function startSSE() {
    const es = new EventSource('/log');
    es.onopen = () => setConnected(true);
    es.onerror = () => setConnected(false);
    es.onmessage = (e) => {
        const item = JSON.parse(e.data);
        item.date = new Date(item.timestamp);
        logBuf.push(item);
        stats.total++;
        stats.recent.push(item.date.getTime());
        if (!item.acl)        stats.aclBlocked++;
        else if (item.blocked) stats.blocked++;
        else if (item.error)   stats.errors++;
        renderStats();
        if (currentTab === 'log') scheduleLogRender();
    };
}

// ── Navigation ────────────────────────────────────────────────────────────────

let currentTab = 'status';

function showTab(id) {
    document.querySelectorAll('section.tab').forEach(s => { s.hidden = true; });
    document.querySelectorAll('nav button[data-tab]').forEach(b => b.classList.remove('active'));
    document.getElementById('tab-' + id).hidden = false;
    document.querySelector(`nav button[data-tab="${id}"]`).classList.add('active');
    currentTab = id;
    if (id === 'status')    loadStatus();
    if (id === 'blocklist') loadBlocklist();
    if (id === 'cache')     loadCache();
    if (id === 'log')       scheduleLogRender();
}

// ── RPC helper ────────────────────────────────────────────────────────────────

async function rpcCall(method, params) {
    const r = await rpc.call(method, params);
    if (r.error) throw new Error(r.error.message ?? JSON.stringify(r.error));
    return r.result;
}

// ── Status tab ────────────────────────────────────────────────────────────────

async function loadStatus() {
    const pre = document.getElementById('config-pre');
    try {
        const result = await rpcCall('api.Config', {});
        pre.textContent = JSON.stringify(result, null, 2);
    } catch (e) {
        pre.textContent = 'Error: ' + e.message;
    }
    try {
        const result = await rpcCall('api.BlockListCount', {});
        setText('status-blcount', result.count);
    } catch (e) {
        setText('status-blcount', 'error');
    }
}

// ── Log tab ───────────────────────────────────────────────────────────────────

const RCODES = { 0:'NOERROR', 1:'FORMERR', 2:'SERVFAIL', 3:'NXDOMAIN', 4:'NOTIMP', 5:'REFUSED' };

let logPaused = false;
let pausedPos = 0;
let logVisible = 20;
let logFilter = null;
let logPending = false;

function formatStatus(item) {
    if (!item.acl)     return 'acl';
    if (item.blocked)  return 'blocked';
    if (item.error)    return 'error';
    if (item.cached)   return 'cached';
    return 'ok';
}

function buildFilter() {
    const defs = [
        { id: 'f-date',   test: (item, re) => re.test(item.date.toISOString()) },
        { id: 'f-client', test: (item, re) => re.test(item.client ?? '') },
        { id: 'f-qname',  test: (item, re) => re.test(item.qname ?? '') },
        { id: 'f-qtype',  test: (item, re) => re.test(item.qtype ?? '') },
        { id: 'f-rcode',  test: (item, re) => re.test(RCODES[item.rcode] ?? String(item.rcode)) },
        { id: 'f-status', test: (item, re) => re.test(formatStatus(item)) },
    ];
    const fns = [];
    for (const { id, test } of defs) {
        const val = document.getElementById(id)?.value ?? '';
        if (!val) continue;
        try {
            const re = new RegExp(val, 'i');
            fns.push(item => test(item, re));
        } catch { /* invalid regex */ }
    }
    logFilter = fns.length ? item => fns.every(fn => fn(item)) : null;
}

function scheduleLogRender() {
    if (!logPending) {
        logPending = true;
        setTimeout(renderLog, 100);
    }
}

function renderLog() {
    logPending = false;
    if (currentTab !== 'log') return;
    const pos = logPaused ? pausedPos : undefined;
    const rows = logBuf.filter(logVisible, logFilter, pos);
    const tbody = document.getElementById('log-tbody');
    tbody.innerHTML = '';
    for (const item of rows) {
        const status = formatStatus(item);
        const tr = tbody.insertRow();
        if (status === 'blocked' || status === 'acl') tr.className = 'blocked';
        else if (status === 'error')  tr.className = 'error';
        else if (status === 'cached') tr.className = 'cached';

        tr.insertCell().textContent = item.date.toTimeString().slice(0, 8);
        tr.insertCell().textContent = item.client ?? '';

        const qcell = tr.insertCell();
        qcell.textContent = item.qname ?? '';
        if (item.acl && !item.blocked) {
            const btn = document.createElement('button');
            btn.className = 'btn-sm blk';
            btn.textContent = 'block';
            btn.onclick = () => quickBlock(item.qname, btn);
            qcell.appendChild(document.createTextNode(' '));
            qcell.appendChild(btn);
        }

        tr.insertCell().textContent = item.qtype ?? '';
        tr.insertCell().textContent = RCODES[item.rcode] ?? item.rcode ?? '';
        tr.insertCell().textContent = status;
        tr.insertCell().textContent = item.querytime ? (item.querytime * 1000).toFixed(1) : '';
    }
    setText('log-count', `${logBuf.length} buffered`);
}

async function quickBlock(qname, btn) {
    btn.disabled = true;
    try {
        await rpcCall('api.BlockListAdd', { entries: [qname] });
        btn.textContent = 'blocked';
    } catch (e) {
        btn.textContent = 'err';
        btn.disabled = false;
    }
}

// ── Blocklist tab ─────────────────────────────────────────────────────────────

async function loadBlocklist() {
    try {
        const result = await rpcCall('api.BlockListCount', {});
        setText('bl-count', result.count);
    } catch (e) {
        setText('bl-count', 'error');
    }
}

async function addBlocklistEntries() {
    const ta = document.getElementById('bl-add');
    const entries = ta.value.split('\n').map(s => s.trim()).filter(Boolean);
    if (!entries.length) return;
    const msg = document.getElementById('bl-msg');
    try {
        await rpcCall('api.BlockListAdd', { entries });
        ta.value = '';
        showMsg(msg, `Added ${entries.length} ${entries.length === 1 ? 'entry' : 'entries'}`, false);
        loadBlocklist();
    } catch (e) {
        showMsg(msg, 'Error: ' + e.message, true);
    }
}

async function deleteBlocklistEntry() {
    const inp = document.getElementById('bl-del');
    const name = inp.value.trim();
    if (!name) return;
    const msg = document.getElementById('bl-msg');
    try {
        const result = await rpcCall('api.BlockListDelete', { name });
        inp.value = '';
        showMsg(msg, result.found ? `Deleted: ${name}` : `Not found: ${name}`, !result.found);
        loadBlocklist();
    } catch (e) {
        showMsg(msg, 'Error: ' + e.message, true);
    }
}

// ── Cache tab ─────────────────────────────────────────────────────────────────

async function loadCache() {
    const tbody = document.getElementById('cache-tbody');
    try {
        const result = await rpcCall('api.CacheDebug', {});
        const entries = (result.entries ?? []).sort();
        setText('cache-count', entries.length);
        tbody.innerHTML = '';
        for (const s of entries) {
            const m = s.match(/^<(.+) (\w+)> (.+)$/);
            if (!m) continue;
            const [, name, qtype, ttl] = m;
            const tr = tbody.insertRow();
            tr.insertCell().textContent = name;
            tr.insertCell().textContent = qtype;
            tr.insertCell().textContent = ttl;
            const cell = tr.insertCell();
            const btn = document.createElement('button');
            btn.className = 'btn-sm';
            btn.textContent = 'delete';
            btn.onclick = async () => {
                btn.disabled = true;
                try {
                    await rpcCall('api.CacheDelete', { name, qtype, ptr: false });
                    tr.remove();
                    const el = document.getElementById('cache-count');
                    el.textContent = parseInt(el.textContent) - 1;
                } catch (e) {
                    btn.disabled = false;
                }
            };
            cell.appendChild(btn);
        }
    } catch (e) {
        const tr = tbody.insertRow();
        const cell = tr.insertCell();
        cell.colSpan = 4;
        cell.className = 'msg-err';
        cell.textContent = 'Error: ' + e.message;
    }
}

async function addCacheEntry() {
    const inp = document.getElementById('cache-add');
    const entry = inp.value.trim();
    if (!entry) return;
    const msg = document.getElementById('cache-msg');
    try {
        await rpcCall('api.CacheAdd', { entry, permanent: true, ptr: false });
        inp.value = '';
        showMsg(msg, 'Added', false);
        loadCache();
    } catch (e) {
        showMsg(msg, 'Error: ' + e.message, true);
    }
}

// ── Utilities ─────────────────────────────────────────────────────────────────

function setText(id, text) {
    const el = document.getElementById(id);
    if (el) el.textContent = text;
}

function showMsg(el, text, isError) {
    el.textContent = text;
    el.className = isError ? 'msg-err' : 'msg-ok';
    clearTimeout(el._timer);
    el._timer = setTimeout(() => { el.textContent = ''; el.className = ''; }, 4000);
}

// ── Init ──────────────────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
    document.querySelectorAll('nav button[data-tab]').forEach(btn => {
        btn.addEventListener('click', () => showTab(btn.dataset.tab));
    });

    document.getElementById('log-visible').addEventListener('change', e => {
        logVisible = parseInt(e.target.value);
        scheduleLogRender();
    });

    const pauseChk = document.getElementById('log-pause');
    pauseChk.addEventListener('change', () => {
        logPaused = pauseChk.checked;
        if (logPaused) pausedPos = logBuf.getPosition();
        scheduleLogRender();
    });

    document.getElementById('log-prev').addEventListener('click', () => {
        if (!logPaused) return;
        const avail = logBuf.calculateAvailable(pausedPos);
        if (avail - logVisible > 0) pausedPos = logBuf.wrapPos(pausedPos - logVisible);
        scheduleLogRender();
    });

    document.getElementById('log-next').addEventListener('click', () => {
        if (!logPaused) return;
        if (logBuf.calculateAvailable(pausedPos) < logBuf.length) {
            pausedPos = logBuf.wrapPos(pausedPos + logVisible);
        }
        scheduleLogRender();
    });

    document.querySelectorAll('.filter-input').forEach(inp => {
        inp.addEventListener('input', () => { buildFilter(); scheduleLogRender(); });
    });

    document.getElementById('filter-clear').addEventListener('click', () => {
        document.querySelectorAll('.filter-input').forEach(inp => { inp.value = ''; });
        logFilter = null;
        scheduleLogRender();
    });

    document.getElementById('bl-refresh-btn').addEventListener('click', loadBlocklist);
    document.getElementById('bl-add-btn').addEventListener('click', addBlocklistEntries);
    document.getElementById('bl-del-btn').addEventListener('click', deleteBlocklistEntry);
    document.getElementById('bl-del').addEventListener('keydown', e => {
        if (e.key === 'Enter') deleteBlocklistEntry();
    });

    document.getElementById('cache-refresh').addEventListener('click', loadCache);
    document.getElementById('cache-add-btn').addEventListener('click', addCacheEntry);
    document.getElementById('cache-add').addEventListener('keydown', e => {
        if (e.key === 'Enter') addCacheEntry();
    });

    startSSE();
    showTab('status');
});

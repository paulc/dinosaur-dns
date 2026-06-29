import { RPC } from './rpc.js';
import { Buffer } from './buffer.js';

const rpc = new RPC('/api');
const logBuf = new Buffer(1000);

// --- Stats ---

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

// --- Connection ---

function setConnected(ok) {
    const el = document.getElementById('indicator');
    el.className = 'indicator ' + (ok ? 'on' : 'off');
    el.title = ok ? 'SSE connected' : 'SSE disconnected';
}

// --- SSE ---

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

// --- Navigation ---

let currentTab = 'log';

function showTab(id) {
    document.querySelectorAll('section.tab').forEach(s => { s.hidden = true; });
    document.querySelectorAll('nav button[data-tab]').forEach(b => b.classList.remove('active'));
    document.getElementById('tab-' + id).hidden = false;
    document.querySelector(`nav button[data-tab="${id}"]`).classList.add('active');
    currentTab = id;
    if (id === 'config')    loadConfig();
    if (id === 'blocklist') loadBlocklist();
    if (id === 'cache')     loadCache();
    if (id === 'log')       scheduleLogRender();
}

// --- RPC helper ---

async function rpcCall(method, params) {
    const r = await rpc.call(method, params);
    if (r.error) throw new Error(r.error.message ?? JSON.stringify(r.error));
    return r.result;
}

// --- Config tab ---

async function loadConfig() {
    const pre = document.getElementById('config-pre');
    try {
        const result = await rpcCall('api.Config', {});
        pre.textContent = JSON.stringify(result, null, 2);
    } catch (e) {
        pre.textContent = 'Error: ' + e.message;
    }
}

// --- Log tab ---

const RCODES = { 0:'NOERROR', 1:'FORMERR', 2:'SERVFAIL', 3:'NXDOMAIN', 4:'NOTIMP', 5:'REFUSED' };

// Track domains blocked this session so the button state survives tab switches
const blockedDomains = new Set();

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
            const alreadyBlocked = blockedDomains.has(item.qname);
            const btn = document.createElement('button');
            btn.className = 'btn-sm blk';
            btn.textContent = alreadyBlocked ? 'blocked' : 'block';
            btn.disabled = alreadyBlocked;
            if (!alreadyBlocked) btn.onclick = () => quickBlock(item.qname, btn);
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
        blockedDomains.add(qname);
        btn.textContent = 'blocked';
        scheduleLogRender(); // update all other rows with the same qname
    } catch (e) {
        btn.textContent = 'err';
        btn.disabled = false;
    }
}

// --- Blocklist tab ---

let blEntries = [];
let blFilterText = '';
let blSort = { col: 'name', dir: 1 };

async function loadBlocklist() {
    try {
        const result = await rpcCall('api.BlockListList', {});
        blEntries = [];
        for (const entry of result.entries ?? []) {
            const name = entry.name.replace(/\.$/, '');
            for (const qtype of entry.block) {
                blEntries.push({ name, qtype });
            }
        }
        renderBlocklist();
    } catch (e) {
        const tbody = document.getElementById('bl-tbody');
        tbody.innerHTML = '';
        const tr = tbody.insertRow();
        const cell = tr.insertCell();
        cell.colSpan = 3;
        cell.className = 'msg-err';
        cell.textContent = 'Error: ' + e.message;
    }
}

function renderBlocklist() {
    setText('bl-count', blEntries.length);

    let rows = blEntries;
    if (blFilterText) {
        try {
            const re = new RegExp(blFilterText, 'i');
            rows = rows.filter(r => re.test(r.name) || re.test(r.qtype));
        } catch { /* invalid regex */ }
    }

    const { col, dir } = blSort;
    rows = [...rows].sort((a, b) => a[col].localeCompare(b[col]) * dir);

    const nameHdr = document.getElementById('bl-hdr-name');
    const typeHdr = document.getElementById('bl-hdr-type');
    nameHdr.textContent = 'Name' + (col === 'name'  ? (dir > 0 ? ' ^' : ' v') : '');
    typeHdr.textContent = 'Type' + (col === 'qtype' ? (dir > 0 ? ' ^' : ' v') : '');

    const LIMIT = 500;
    const shown = rows.slice(0, LIMIT);
    const tbody = document.getElementById('bl-tbody');
    tbody.innerHTML = '';

    for (const row of shown) {
        const tr = tbody.insertRow();
        tr.insertCell().textContent = row.name;
        tr.insertCell().textContent = row.qtype;
        const cell = tr.insertCell();
        const btn = document.createElement('button');
        btn.className = 'btn-sm';
        btn.textContent = 'delete';
        btn.onclick = async () => {
            btn.disabled = true;
            const delName = row.qtype === 'ANY' ? row.name : `${row.name}:${row.qtype}`;
            try {
                await rpcCall('api.BlockListDelete', { name: delName });
                if (row.qtype === 'ANY') blockedDomains.delete(row.name);
                const idx = blEntries.findIndex(e => e.name === row.name && e.qtype === row.qtype);
                if (idx !== -1) blEntries.splice(idx, 1);
                renderBlocklist();
            } catch (e) {
                btn.disabled = false;
            }
        };
        cell.appendChild(btn);
    }

    if (rows.length > LIMIT) {
        const tr = tbody.insertRow();
        const cell = tr.insertCell();
        cell.colSpan = 3;
        cell.style.color = '#888';
        cell.textContent = `Showing ${LIMIT} of ${rows.length} entries -- refine filter to see more`;
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
        await loadBlocklist();
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
        if (result.found) {
            blockedDomains.delete(name.split(':')[0]);
            await loadBlocklist();
        }
    } catch (e) {
        showMsg(msg, 'Error: ' + e.message, true);
    }
}

// --- Cache tab ---

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
    const rr = inp.value.trim();
    if (!rr) return;
    const msg = document.getElementById('cache-msg');
    try {
        await rpcCall('api.CacheAdd', { rr, permanent: true, ptr: false });
        inp.value = '';
        showMsg(msg, 'Added', false);
        loadCache();
    } catch (e) {
        showMsg(msg, 'Error: ' + e.message, true);
    }
}

// --- Utilities ---

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

// --- Init ---

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
    document.getElementById('bl-filter').addEventListener('input', e => {
        blFilterText = e.target.value;
        renderBlocklist();
    });
    document.getElementById('bl-filter-clear').addEventListener('click', () => {
        document.getElementById('bl-filter').value = '';
        blFilterText = '';
        renderBlocklist();
    });
    document.getElementById('bl-hdr-name').addEventListener('click', () => {
        blSort = blSort.col === 'name' ? { col: 'name', dir: -blSort.dir } : { col: 'name', dir: 1 };
        renderBlocklist();
    });
    document.getElementById('bl-hdr-type').addEventListener('click', () => {
        blSort = blSort.col === 'qtype' ? { col: 'qtype', dir: -blSort.dir } : { col: 'qtype', dir: 1 };
        renderBlocklist();
    });

    document.getElementById('cache-refresh').addEventListener('click', loadCache);
    document.getElementById('cache-add-btn').addEventListener('click', addCacheEntry);
    document.getElementById('cache-add').addEventListener('keydown', e => {
        if (e.key === 'Enter') addCacheEntry();
    });

    startSSE();
    showTab('log');
});

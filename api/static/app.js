import { RPC } from './rpc.js';
import { Buffer } from './buffer.js';

const rpc = new RPC('/api');
const logBuf = new Buffer(1000);

// --- Block pause ---

let pauseEndTime = null;
let pauseCountdownTimer = null;

async function checkBlockingStatus() {
    try {
        const result = await rpcCall('api.GetBlockingStatus', {});
        if (result.paused) {
            pauseEndTime = new Date(Date.now() + result.remaining_seconds * 1000);
            showPauseWarning();
        } else {
            endPauseWarning();
        }
    } catch { /* ignore — server may not be reachable yet */ }
}

function showPauseWarning() {
    document.getElementById('pause-warning').hidden = false;
    updatePauseCountdown();
    if (!pauseCountdownTimer) {
        pauseCountdownTimer = setInterval(() => {
            if (pauseEndTime && Date.now() < pauseEndTime.getTime()) {
                updatePauseCountdown();
            } else {
                endPauseWarning();
            }
        }, 1000);
    }
}

function endPauseWarning() {
    document.getElementById('pause-warning').hidden = true;
    clearInterval(pauseCountdownTimer);
    pauseCountdownTimer = null;
    pauseEndTime = null;
}

function updatePauseCountdown() {
    if (!pauseEndTime) return;
    const remaining = Math.max(0, Math.round((pauseEndTime.getTime() - Date.now()) / 1000));
    const m = Math.floor(remaining / 60);
    const s = remaining % 60;
    setText('pause-countdown', m > 0 ? `${m}m ${s}s` : `${s}s`);
}

// --- Stats ---

const stats = { total: 0, blocked: 0, aclBlocked: 0, errors: 0 };

// Tick-based EMA rate (queries/min). Updated every EMA_TICK ms regardless of traffic.
const EMA_TICK  = 1000;  // ms between ticks
const EMA_ALPHA = 0.5;   // smoothing factor; half-life ~1s
let emaRate    = 0;
let emaTick    = 0;      // query count accumulated since last tick

function renderStats() {
    const pct = n => stats.total ? ` (${(n * 100 / stats.total).toFixed(1)}%)` : '';
    setText('stat-total',   stats.total);
    setText('stat-blocked', `${stats.blocked}${pct(stats.blocked)}`);
    setText('stat-acl',     `${stats.aclBlocked}${pct(stats.aclBlocked)}`);
    setText('stat-errors',  `${stats.errors}${pct(stats.errors)}`);
    setText('stat-rate',    `${emaRate.toFixed(1)}/min`);
}

function tickEMA() {
    const instantRate = emaTick * (60000 / EMA_TICK); // scale tick count to per-minute
    emaRate = EMA_ALPHA * instantRate + (1 - EMA_ALPHA) * emaRate;
    emaTick = 0;
    renderStats();
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
        emaTick++;
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
    if (id === 'config')    { loadConfig(); loadChanges(); }
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

async function loadChanges() {
    try {
        const result = await rpcCall('api.GetChanges', {});
        renderChanges(result);
    } catch (e) {
        document.getElementById('changes-blocks').textContent = 'Error: ' + e.message;
        document.getElementById('changes-rrs').textContent = '';
    }
}

function renderChanges(result) {
    const blocks       = result.blocks ?? [];
    const blockDeletes = result.block_deletes ?? [];
    const rrs          = result.local_rrs ?? [];
    const rrPtrs       = result.local_rr_ptrs ?? [];
    const rrDeletes    = result.local_rr_deletes ?? [];

    const bPre = document.getElementById('changes-blocks');
    const rPre = document.getElementById('changes-rrs');

    const blockLines = [
        ...blocks.map(d => `-block ${d}`),
        ...blockDeletes.map(d => `-block-delete ${d}`),
    ];
    bPre.textContent = blockLines.length ? blockLines.join('\n') : '(none)';

    const rrLines = [
        ...rrs.map(r => `-localrr "${r}"`),
        ...rrPtrs.map(r => `-localrr-ptr "${r}"`),
        ...rrDeletes.map(k => `# deleted: ${k}`),
    ];
    rPre.textContent = rrLines.length ? rrLines.join('\n') : '(none)';
}

// --- Log tab ---

const RCODES = { 0:'NOERROR', 1:'FORMERR', 2:'SERVFAIL', 3:'NXDOMAIN', 4:'NOTIMP', 5:'REFUSED' };

let logPaused = false;
let pausedPos = 0;
let logPageOffset = 0;  // offset in filtered-item space from the anchor; 0 = newest
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
    const anchor = logPaused ? pausedPos : logBuf.getPosition();
    const rows = logBuf.filter(logVisible, logFilter, anchor, logPageOffset);
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

        if (item.acl) {
            if (!item.blocked) {
                const btn = document.createElement('button');
                btn.className = 'btn-sm blk log-btn';
                btn.textContent = 'block';
                btn.onclick = () => quickBlock(item.qname, btn);
                qcell.appendChild(document.createTextNode(' '));
                qcell.appendChild(btn);
            } else {
                const btn = document.createElement('button');
                btn.className = 'btn-sm ublk log-btn';
                btn.textContent = 'unblock';
                btn.onclick = () => quickUnblock(item.qname, btn);
                qcell.appendChild(document.createTextNode(' '));
                qcell.appendChild(btn);
            }
        }

        tr.insertCell().textContent = item.qtype ?? '';
        tr.insertCell().textContent = RCODES[item.rcode] ?? item.rcode ?? '';
        tr.insertCell().textContent = status;
        tr.insertCell().textContent = item.querytime ? (item.querytime * 1000).toFixed(1) : '';
    }
    const totalMatching = logFilter ? logBuf.countFiltered(logFilter, anchor) : logBuf.calculateAvailable(anchor);
    const from = rows.length ? logPageOffset + 1 : 0;
    const to   = logPageOffset + rows.length;
    let countText = logFilter
        ? `${from}-${to} of ${totalMatching} matching  (${logBuf.length} buffered)`
        : `${from}-${to} of ${logBuf.length} buffered`;
    if (!rows.length) countText = logFilter ? `0 matching  (${logBuf.length} buffered)` : `${logBuf.length} buffered`;
    setText('log-count', countText);
}

// Mutate ring buffer entries in place so row colour, status, and button
// all derive from the same item.blocked field rather than a separate set.
function setDomainBlocked(qname, blocked) {
    for (const item of logBuf.tail(logBuf.length)) {
        if (item.qname === qname && item.acl) item.blocked = blocked;
    }
}

async function quickBlock(qname, btn) {
    btn.disabled = true;
    try {
        await rpcCall('api.BlockListAdd', { entries: [qname] });
        setDomainBlocked(qname, true);
        scheduleLogRender();
    } catch (e) {
        btn.textContent = 'err';
        btn.disabled = false;
    }
}

async function quickUnblock(qname, btn) {
    btn.disabled = true;
    try {
        await rpcCall('api.BlockListDelete', { name: qname });
        setDomainBlocked(qname, false);
        scheduleLogRender();
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
                if (row.qtype === 'ANY') setDomainBlocked(row.name, false);
                const idx = blEntries.findIndex(e => e.name === row.name && e.qtype === row.qtype);
                if (idx !== -1) blEntries.splice(idx, 1);
                renderBlocklist();
                scheduleLogRender();
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
            setDomainBlocked(name.split(':')[0], false);
            scheduleLogRender();
            await loadBlocklist();
        }
    } catch (e) {
        showMsg(msg, 'Error: ' + e.message, true);
    }
}

// --- Cache tab ---

async function loadCache() {
    try {
        const result = await rpcCall('api.CacheDebug', {});
        const entries = (result.entries ?? []).sort();

        const permanent = [];
        const cached    = [];
        for (const s of entries) {
            const m = s.match(/^<(.+) (\w+)> (.+)$/);
            if (!m) continue;
            const [, name, qtype, ttl] = m;
            if (ttl === 'permanent') permanent.push({ name, qtype, ttl });
            else                     cached.push({ name, qtype, ttl });
        }

        setText('cache-perm-count',   permanent.length);
        setText('cache-cached-count', cached.length);
        renderCacheTable(permanent, 'cache-perm-tbody',   true);
        renderCacheTable(cached,    'cache-cached-tbody', false);
    } catch (e) {
        const tr = document.getElementById('cache-perm-tbody').insertRow();
        const cell = tr.insertCell();
        cell.colSpan = 4;
        cell.className = 'msg-err';
        cell.textContent = 'Error: ' + e.message;
    }
}

function renderCacheTable(entries, tbodyId, isPerm) {
    const tbody = document.getElementById(tbodyId);
    tbody.innerHTML = '';
    for (const { name, qtype, ttl } of entries) {
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
                const countId = isPerm ? 'cache-perm-count' : 'cache-cached-count';
                const el = document.getElementById(countId);
                el.textContent = parseInt(el.textContent) - 1;
            } catch (e) {
                btn.disabled = false;
            }
        };
        cell.appendChild(btn);
    }
}

async function addCacheEntry(withPtr) {
    const inp = document.getElementById('cache-add');
    const rr = inp.value.trim();
    if (!rr) return;
    const msg = document.getElementById('cache-msg');
    try {
        await rpcCall('api.CacheAdd', { rr, permanent: true, ptr: withPtr });
        inp.value = '';
        showMsg(msg, withPtr ? 'Added with PTR' : 'Added', false);
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
        logPageOffset = 0;
        scheduleLogRender();
    });

    document.getElementById('log-prev').addEventListener('click', () => {
        if (!logPaused) {
            logPaused = true;
            pauseChk.checked = true;
            pausedPos = logBuf.getPosition();
            logPageOffset = 0;
        }
        const anchor = pausedPos;
        const total = logFilter ? logBuf.countFiltered(logFilter, anchor) : logBuf.calculateAvailable(anchor);
        if (logPageOffset + logVisible < total) logPageOffset += logVisible;
        scheduleLogRender();
    });

    document.getElementById('log-next').addEventListener('click', () => {
        if (!logPaused) return;
        logPageOffset = Math.max(0, logPageOffset - logVisible);
        scheduleLogRender();
    });

    document.querySelectorAll('.filter-input').forEach(inp => {
        inp.addEventListener('input', () => { buildFilter(); logPageOffset = 0; scheduleLogRender(); });
    });

    document.getElementById('filter-clear').addEventListener('click', () => {
        document.querySelectorAll('.filter-input').forEach(inp => { inp.value = ''; });
        logFilter = null;
        logPageOffset = 0;
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
    document.getElementById('cache-add-btn').addEventListener('click', () => addCacheEntry(false));
    document.getElementById('cache-add-ptr-btn').addEventListener('click', () => addCacheEntry(true));
    document.getElementById('cache-add').addEventListener('keydown', e => {
        if (e.key === 'Enter') addCacheEntry(false);
    });

    document.getElementById('merge-config-btn').addEventListener('click', async () => {
        const btn = document.getElementById('merge-config-btn');
        const msg = document.getElementById('merge-msg');
        const pre = document.getElementById('merged-config-pre');
        btn.disabled = true;
        try {
            const result = await rpcCall('api.GetMergedConfig', {});
            pre.textContent = result.config;
            pre.hidden = false;
            showMsg(msg, 'Generated', false);
        } catch (e) {
            showMsg(msg, 'Error: ' + e.message, true);
        } finally {
            btn.disabled = false;
        }
    });

    document.getElementById('pause-btn').addEventListener('click', async () => {
        const seconds = parseInt(document.getElementById('pause-seconds').value) || 300;
        try {
            const result = await rpcCall('api.PauseBlocking', { seconds });
            if (result.paused) {
                pauseEndTime = new Date(Date.now() + result.remaining_seconds * 1000);
                showPauseWarning();
            }
            showMsg(document.getElementById('bl-pause-msg'), `Paused for ${seconds}s`, false);
        } catch (e) {
            showMsg(document.getElementById('bl-pause-msg'), 'Error: ' + e.message, true);
        }
    });

    const resumeHandler = async () => {
        try {
            await rpcCall('api.ResumeBlocking', {});
            endPauseWarning();
        } catch (e) {
            showMsg(document.getElementById('bl-pause-msg'), 'Error: ' + e.message, true);
        }
    };
    document.getElementById('resume-btn').addEventListener('click', resumeHandler);
    document.getElementById('pause-resume-quick').addEventListener('click', resumeHandler);

    setInterval(checkBlockingStatus, 5000);
    checkBlockingStatus();
    setInterval(tickEMA, EMA_TICK);
    startSSE();
    showTab('log');
});

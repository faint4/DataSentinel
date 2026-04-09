// DataSentinel 前端逻辑
const state = {
    scanning: false,
    results: [],
    filtered: [],
    summary: null,
    activeTab: 'results',
    filterLevel: 0,
    sortBy: 'level',
    sortDir: 'desc',
    report: null,
    page: 1,
    pageSize: 50,
    selectedPaths: new Set(),
};

const levelNames = { 1: 'L1', 2: 'L2', 3: 'L3', 4: 'L4', 5: 'L5' };
const levelColors = { 1: '#22c55e', 2: '#3b82f6', 3: '#f59e0b', 4: '#ef4444', 5: '#7c3aed' };

// Map backend by_level keys (e.g. "L1-Public") to numeric level
function levelKeyToNum(key) {
    return parseInt(key.replace(/\D/g, '')) || 0;
}
const categoryNames = {
    id_card: '身份证号', phone: '手机号', bank_card: '银行卡号',
    email: '邮箱', ip: 'IP地址', uscc: '统一社会信用代码',
    aws_key: 'AWS密钥', github_token: 'GitHub Token',
    private_key: '私钥', high_entropy: '高熵密钥'
};

// === Initialization ===
document.addEventListener('DOMContentLoaded', () => {
    initExtCheckboxes();
    initRuleCheckboxes();
    initTabs();
    initTheme();
    initEventHandlers();
});

function initExtCheckboxes() {
    const container = document.getElementById('extCheckboxes');
    const exts = ['.txt','.csv','.log','.json','.xml','.md','.env','.yaml','.yml','.ini','.conf','.toml','.bat','.ps1','.sh','.sql','.docx','.xlsx','.pptx'];
    const frag = document.createDocumentFragment();
    exts.forEach(ext => {
        const label = document.createElement('label');
        label.className = 'ext-item';
        const cb = document.createElement('input');
        cb.type = 'checkbox'; cb.value = ext; cb.checked = true;
        label.appendChild(cb);
        label.appendChild(document.createTextNode(ext));
        frag.appendChild(label);
    });
    container.appendChild(frag);
}

function initRuleCheckboxes() {
    const container = document.getElementById('ruleCheckboxes');
    const rules = [
        {id:'id_card', name:'身份证号'}, {id:'phone', name:'手机号'}, {id:'bank_card', name:'银行卡号'},
        {id:'email', name:'邮箱地址'}, {id:'ip', name:'IP地址'}, {id:'uscc', name:'社会信用代码'},
        {id:'aws_key', name:'AWS密钥'}, {id:'github_token', name:'GitHub Token'},
        {id:'private_key', name:'私钥文件'}, {id:'high_entropy', name:'高熵密钥'}
    ];
    const frag = document.createDocumentFragment();
    rules.forEach(rule => {
        const label = document.createElement('label');
        label.className = 'ext-item rule-item';
        const cb = document.createElement('input');
        cb.type = 'checkbox'; cb.value = rule.id; cb.checked = true;
        label.appendChild(cb);
        label.appendChild(document.createTextNode(rule.name));
        frag.appendChild(label);
    });
    container.appendChild(frag);
}

function initTabs() {
    document.querySelectorAll('.tab').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.tab').forEach(b => b.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
            btn.classList.add('active');
            document.getElementById('tab-' + btn.dataset.tab).classList.add('active');
        });
    });
}

function initTheme() {
    const saved = localStorage.getItem('datasentinel-theme') || 'light';
    document.body.setAttribute('data-theme', saved);
    document.getElementById('themeToggle').textContent = saved === 'dark' ? '\u2600' : '\u263E';
}

function initEventHandlers() {
    document.getElementById('themeToggle').addEventListener('click', () => {
        const cur = document.body.getAttribute('data-theme');
        const next = cur === 'dark' ? 'light' : 'dark';
        document.body.setAttribute('data-theme', next);
        localStorage.setItem('datasentinel-theme', next);
        document.getElementById('themeToggle').textContent = next === 'dark' ? '\u2600' : '\u263E';
    });

    document.getElementById('startScan').addEventListener('click', startScan);
    document.getElementById('cancelScan').addEventListener('click', cancelScan);
    document.getElementById('selectAll').addEventListener('click', () => toggleAll('#extCheckboxes input', true));
    document.getElementById('deselectAll').addEventListener('click', () => toggleAll('#extCheckboxes input', false));
    document.getElementById('selectAllRules').addEventListener('click', () => toggleAll('#ruleCheckboxes input', true));
    document.getElementById('deselectAllRules').addEventListener('click', () => toggleAll('#ruleCheckboxes input', false));
    document.getElementById('sortSelect').addEventListener('change', e => { state.sortBy = e.target.value; rebuildFiltered(); });
    document.getElementById('pageSizeSelect').addEventListener('change', e => { state.pageSize = parseInt(e.target.value); state.page = 1; renderPage(); });

    document.querySelectorAll('.filter-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            state.filterLevel = parseInt(btn.dataset.level);
            state.page = 1;
            rebuildFiltered();
        });
    });

    document.getElementById('exportJSON').addEventListener('click', () => downloadReport('json'));
    document.getElementById('exportCSV').addEventListener('click', () => downloadReport('csv'));

    document.getElementById('modalClose').addEventListener('click', () => {
        document.getElementById('detailModal').style.display = 'none';
    });
    // Close detail modal on ESC key
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            document.getElementById('detailModal').style.display = 'none';
        }
    });
    // Close detail modal on backdrop click
    document.getElementById('detailModal').addEventListener('click', (e) => {
        if (e.target === e.currentTarget) {
            e.currentTarget.style.display = 'none';
        }
    });

    // Pagination
    document.getElementById('pageFirst').addEventListener('click', () => { state.page = 1; renderPage(); });
    document.getElementById('pagePrev').addEventListener('click', () => { if (state.page > 1) { state.page--; renderPage(); } });
    document.getElementById('pageNext').addEventListener('click', () => { const max = totalPages(); if (state.page < max) { state.page++; renderPage(); } });
    document.getElementById('pageLast').addEventListener('click', () => { state.page = totalPages(); renderPage(); });

    // Browse directory
    document.getElementById('browseBtn').addEventListener('click', () => openBrowseModal());
    document.getElementById('browseClose').addEventListener('click', () => {
        document.getElementById('browseModal').style.display = 'none';
    });
    document.getElementById('browseCancelBtn').addEventListener('click', () => {
        document.getElementById('browseModal').style.display = 'none';
    });
    document.getElementById('browseConfirmBtn').addEventListener('click', () => {
        const path = document.getElementById('browseCurrentPath').textContent;
        document.getElementById('scanPath').value = path;
        document.getElementById('browseModal').style.display = 'none';
    });
    document.getElementById('browseGoBtn').addEventListener('click', () => {
        const path = document.getElementById('browsePathInput').value.trim();
        if (path) loadBrowseDir(path);
    });
    document.getElementById('browsePathInput').addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
            const path = e.target.value.trim();
            if (path) loadBrowseDir(path);
        }
    });

    // Help modal
    document.getElementById('helpBtn').addEventListener('click', () => {
        document.getElementById('helpModal').style.display = 'flex';
    });
    document.getElementById('helpClose').addEventListener('click', () => {
        document.getElementById('helpModal').style.display = 'none';
    });
    document.querySelectorAll('.help-nav-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.help-nav-btn').forEach(b => b.classList.remove('active'));
            document.querySelectorAll('.help-section').forEach(s => s.classList.remove('active'));
            btn.classList.add('active');
            document.getElementById('help' + btn.dataset.section.charAt(0).toUpperCase() + btn.dataset.section.slice(1)).classList.add('active');
        });
    });

    // Welcome modal (first run)
    if (!localStorage.getItem('datasentinel-welcomed')) {
        document.getElementById('welcomeModal').style.display = 'flex';
    }
    document.getElementById('welcomeStartBtn').addEventListener('click', () => {
        if (document.getElementById('dontShowAgain').checked) {
            localStorage.setItem('datasentinel-welcomed', '1');
        }
        document.getElementById('welcomeModal').style.display = 'none';
    });

    // Batch toolbar
    document.getElementById('selectAllRows').addEventListener('change', (e) => {
        const checked = e.target.checked;
        document.querySelectorAll('.row-check').forEach(cb => {
            cb.checked = checked;
            const path = cb.dataset.path;
            if (checked) state.selectedPaths.add(path);
            else state.selectedPaths.delete(path);
        });
        updateBatchToolbar();
    });
    document.getElementById('batchCorrectBtn').addEventListener('click', () => {
        if (state.selectedPaths.size === 0) return;
        document.getElementById('correctTitle').textContent = `批量修正 (${state.selectedPaths.size} 个文件)`;
        document.getElementById('correctLevel').value = '1';
        document.getElementById('correctNote').value = '';
        document.getElementById('correctModal').style.display = 'flex';
    });

    // Correction modal
    document.getElementById('correctClose').addEventListener('click', () => {
        document.getElementById('correctModal').style.display = 'none';
    });
    document.getElementById('correctCancelBtn').addEventListener('click', () => {
        document.getElementById('correctModal').style.display = 'none';
    });
    document.getElementById('correctConfirmBtn').addEventListener('click', submitCorrection);
    document.getElementById('correctModal').addEventListener('click', (e) => {
        if (e.target === e.currentTarget) e.currentTarget.style.display = 'none';
    });
}

function toggleAll(selector, checked) {
    document.querySelectorAll(selector).forEach(cb => cb.checked = checked);
}

// === Scan Control ===
function getSelected(selector) {
    return Array.from(document.querySelectorAll(selector + ':checked')).map(cb => cb.value);
}

async function startScan() {
    const path = document.getElementById('scanPath').value.trim();
    if (!path) { alert('请输入扫描目录路径'); return; }
    const exts = getSelected('#extCheckboxes input');
    if (exts.length === 0) { alert('请至少选择一种文件类型'); return; }
    const categories = getSelected('#ruleCheckboxes input');
    if (categories.length === 0) { alert('请至少选择一种检测策略'); return; }

    state.scanning = true;
    state.results = [];
    state.filtered = [];
    state.summary = null;
    state.report = null;
    state.page = 1;
    updateScanUI(true);
    showLoadingState();

    try {
        const resp = await fetch('/api/scan/start', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path, extensions: exts, categories })
        });
        if (!resp.ok) {
            const err = await resp.json();
            alert(err.error || '启动扫描失败');
            state.scanning = false;
            updateScanUI(false);
            return;
        }
        connectProgress();
    } catch (e) {
        alert('网络错误: ' + e.message);
        state.scanning = false;
        updateScanUI(false);
    }
}

async function cancelScan() {
    try { await fetch('/api/scan/cancel', { method: 'POST' }); } catch (e) { /* ignore */ }
}

function connectProgress() {
    const es = new EventSource('/api/scan/status');
    let count = 0;
    es.onmessage = (e) => {
        const evt = JSON.parse(e.data);
        switch (evt.type) {
            case 'file_done':
                // Only accumulate in memory, update live counter
                if (evt.file_result) {
                    state.results.push(evt.file_result);
                    count++;
                    document.getElementById('liveCount').textContent = count;
                }
                updateProgress(evt);
                break;
            case 'scan_done':
                state.scanning = false;
                updateScanUI(false);
                es.close();
                fetchResults();
                break;
            case 'error':
                alert('扫描错误: ' + (evt.message || '未知错误'));
                state.scanning = false;
                updateScanUI(false);
                es.close();
                break;
        }
    };
    es.onerror = () => {
        state.scanning = false;
        updateScanUI(false);
        es.close();
    };
}

async function fetchResults() {
    try {
        const resp = await fetch('/api/results');
        const data = await resp.json();
        state.results = data.results || [];
        state.summary = data.summary || null;
        state.report = data;
        state.page = 1;
        state.selectedPaths.clear();
        renderSummaryBar();
        rebuildFiltered();
        renderStats();
        renderReport();
        document.getElementById('batchToolbar').style.display = state.results.length > 0 ? '' : 'none';
    } catch (e) { /* ignore */ }
}

// === Summary Bar (always visible in results tab) ===
function renderSummaryBar() {
    const sum = state.summary;
    if (!sum) return;
    document.getElementById('resultsSummary').style.display = '';
    document.getElementById('rsTotal').textContent = sum.total_files;
    const sensitive = Object.entries(sum.by_level || {})
        .filter(([k]) => levelKeyToNum(k) > 1)
        .reduce((s, [, v]) => s + v, 0);
    document.getElementById('rsSensitive').textContent = sensitive;
    document.getElementById('rsMatches').textContent = sum.total_matches;
    const highest = Object.entries(sum.by_level || {})
        .filter(([, v]) => v > 0)
        .map(([k]) => levelKeyToNum(k))
        .reduce((a, b) => Math.max(a, b), 0);
    document.getElementById('rsHighest').textContent = highest > 0 ? levelNames[highest] : '-';
}

// === UI States ===
function updateScanUI(scanning) {
    document.getElementById('startScan').disabled = scanning;
    document.getElementById('cancelScan').disabled = !scanning;
    document.getElementById('progressFill').style.width = scanning ? '0%' : '100%';
    document.getElementById('progressText').textContent = scanning ? '扫描中...' : '就绪';
}

function updateProgress(evt) {
    if (evt.total > 0) {
        const pct = Math.round((evt.completed / evt.total) * 100);
        document.getElementById('progressFill').style.width = pct + '%';
        document.getElementById('progressText').textContent = pct + '%';
    }
    document.getElementById('progressDetail').textContent = evt.current || '';
}

function showLoadingState() {
    document.getElementById('resultsEmpty').style.display = 'none';
    document.getElementById('resultsTable').style.display = 'none';
    document.getElementById('pagination').style.display = 'none';
    document.getElementById('resultsLoading').style.display = '';
    document.getElementById('resultsSummary').style.display = 'none';
    document.getElementById('filteredCount').textContent = '';
    document.getElementById('liveCount').textContent = '0';
}

// === Filtering + Sorting + Pagination ===
function rebuildFiltered() {
    let filtered = state.results;
    if (state.filterLevel > 0) {
        filtered = filtered.filter(r => r.level === state.filterLevel);
    }
    // Sort
    filtered = [...filtered].sort((a, b) => {
        let va, vb;
        switch (state.sortBy) {
            case 'level': va = a.level; vb = b.level; break;
            case 'path': va = a.path; vb = b.path; break;
            case 'matches': va = (a.matches||[]).length; vb = (b.matches||[]).length; break;
            case 'size': va = a.size; vb = b.size; break;
            default: va = a.level; vb = b.level;
        }
        if (typeof va === 'string') return state.sortDir === 'asc' ? va.localeCompare(vb) : vb.localeCompare(va);
        return state.sortDir === 'asc' ? va - vb : vb - va;
    });
    state.filtered = filtered;

    document.getElementById('filteredCount').textContent =
        filtered.length === state.results.length ? '' : `${filtered.length} / ${state.results.length}`;

    if (filtered.length === 0 && state.results.length > 0) {
        document.getElementById('resultsEmpty').style.display = 'none';
        document.getElementById('resultsLoading').style.display = 'none';
        document.getElementById('resultsTable').style.display = 'none';
        document.getElementById('pagination').style.display = 'none';
        document.getElementById('filteredCount').textContent = '无匹配结果';
        return;
    }

    renderPage();
}

function totalPages() {
    return Math.max(1, Math.ceil(state.filtered.length / state.pageSize));
}

function renderPage() {
    const filtered = state.filtered;
    const tbody = document.getElementById('resultsBody');
    const total = filtered.length;
    const pages = totalPages();

    if (total === 0 && state.results.length === 0) {
        // No scan done yet
        document.getElementById('resultsEmpty').style.display = '';
        document.getElementById('resultsLoading').style.display = 'none';
        document.getElementById('resultsTable').style.display = 'none';
        document.getElementById('pagination').style.display = 'none';
        return;
    }

    // Hide loading/empty, show table
    document.getElementById('resultsEmpty').style.display = 'none';
    document.getElementById('resultsLoading').style.display = 'none';
    document.getElementById('resultsTable').style.display = '';

    // Clamp page
    if (state.page > pages) state.page = pages;
    if (state.page < 1) state.page = 1;

    const start = (state.page - 1) * state.pageSize;
    const end = Math.min(start + state.pageSize, total);
    const slice = filtered.slice(start, end);

    // Build rows with DocumentFragment
    const frag = document.createDocumentFragment();
    slice.forEach(fr => frag.appendChild(createResultRow(fr)));
    tbody.innerHTML = '';
    tbody.appendChild(frag);

    // Pagination
    const pagination = document.getElementById('pagination');
    if (total <= state.pageSize) {
        pagination.style.display = 'none';
    } else {
        pagination.style.display = '';
        document.getElementById('pageInfo').textContent = `第 ${start+1}-${end} 条，共 ${total} 条`;
        document.getElementById('pagePrev').disabled = state.page <= 1;
        document.getElementById('pageFirst').disabled = state.page <= 1;
        document.getElementById('pageNext').disabled = state.page >= pages;
        document.getElementById('pageLast').disabled = state.page >= pages;
        renderPageNumbers(pages);
    }
}

function renderPageNumbers(totalPages) {
    const container = document.getElementById('pageNumbers');
    container.innerHTML = '';
    const cur = state.page;
    const range = 2; // show cur-2 .. cur .. cur+2
    let start = Math.max(1, cur - range);
    let end = Math.min(totalPages, cur + range);

    // Ensure at least 5 pages shown when possible
    if (end - start < 4) {
        if (start === 1) end = Math.min(totalPages, start + 4);
        else start = Math.max(1, end - 4);
    }

    if (start > 1) {
        container.appendChild(makePageBtn(1));
        if (start > 2) container.appendChild(document.createTextNode('...'));
    }
    for (let i = start; i <= end; i++) {
        container.appendChild(makePageBtn(i));
    }
    if (end < totalPages) {
        if (end < totalPages - 1) container.appendChild(document.createTextNode('...'));
        container.appendChild(makePageBtn(totalPages));
    }
}

function makePageBtn(n) {
    const btn = document.createElement('button');
    btn.className = 'page-btn' + (n === state.page ? ' active' : '');
    btn.textContent = n;
    btn.addEventListener('click', () => { state.page = n; renderPage(); });
    return btn;
}

function createResultRow(fr) {
    const tr = document.createElement('tr');
    const effectiveLevel = fr.corrected_level || fr.level;
    tr.dataset.level = effectiveLevel;
    if (fr.corrected_level) tr.classList.add('corrected');

    // Checkbox
    const tdCheck = document.createElement('td');
    const cb = document.createElement('input');
    cb.type = 'checkbox';
    cb.className = 'row-check';
    cb.dataset.path = fr.path;
    cb.checked = state.selectedPaths.has(fr.path);
    cb.addEventListener('change', (e) => {
        if (e.target.checked) state.selectedPaths.add(fr.path);
        else state.selectedPaths.delete(fr.path);
        updateBatchToolbar();
    });
    tdCheck.appendChild(cb);

    // Level badge
    const badge = document.createElement('span');
    badge.className = 'level-badge';
    badge.style.background = levelColors[effectiveLevel] || '#999';
    badge.textContent = levelNames[effectiveLevel] || '?';

    const tdLevel = document.createElement('td');
    tdLevel.appendChild(badge);
    if (fr.corrected_level) {
        const origBadge = document.createElement('span');
        origBadge.className = 'orig-badge';
        origBadge.textContent = '(原' + (levelNames[fr.level] || '?') + ')';
        origBadge.title = '原级别: ' + (levelNames[fr.level] || '?') + (fr.correction_note ? ' - ' + fr.correction_note : '');
        tdLevel.appendChild(origBadge);
    }

    const tdPath = document.createElement('td');
    tdPath.className = 'path-cell';
    tdPath.title = fr.path;
    tdPath.textContent = fr.path;

    const tdExt = document.createElement('td');
    tdExt.textContent = fr.ext;

    const tdSize = document.createElement('td');
    tdSize.textContent = formatSize(fr.size);

    const tdMatches = document.createElement('td');
    tdMatches.textContent = fr.matches ? fr.matches.length : 0;

    const tdCats = document.createElement('td');
    tdCats.className = 'cats-cell';
    if (fr.matches && fr.matches.length > 0) {
        const seen = new Set();
        fr.matches.forEach(m => {
            if (!seen.has(m.category)) {
                seen.add(m.category);
                const tag = document.createElement('span');
                tag.className = 'cat-tag';
                tag.textContent = categoryNames[m.category] || m.category;
                tdCats.appendChild(tag);
            }
        });
    }

    const tdAction = document.createElement('td');
    const btn = document.createElement('button');
    btn.className = 'btn-view';
    btn.textContent = '查看';
    btn.addEventListener('click', () => showDetail(fr));
    tdAction.appendChild(btn);
    // Single-row correct button
    const btnCorr = document.createElement('button');
    btnCorr.className = 'btn-correct-sm';
    btnCorr.textContent = '修正';
    btnCorr.addEventListener('click', () => {
        state.selectedPaths.clear();
        state.selectedPaths.add(fr.path);
        document.getElementById('correctTitle').textContent = '修正分类 - ' + (fr.name || fr.path);
        document.getElementById('correctLevel').value = fr.corrected_level || fr.level;
        document.getElementById('correctNote').value = fr.correction_note || '';
        document.getElementById('correctModal').style.display = 'flex';
    });
    tdAction.appendChild(btnCorr);

    tr.appendChild(tdCheck);
    tr.appendChild(tdLevel);
    tr.appendChild(tdPath);
    tr.appendChild(tdExt);
    tr.appendChild(tdSize);
    tr.appendChild(tdMatches);
    tr.appendChild(tdCats);
    tr.appendChild(tdAction);
    return tr;
}

function showDetail(fr) {
    const modal = document.getElementById('detailModal');
    document.getElementById('modalTitle').textContent = fr.name || fr.path;
    const body = document.getElementById('modalBody');
    if (!fr.matches || fr.matches.length === 0) {
        body.innerHTML = '<p>无匹配数据</p>';
    } else {
        const frag = document.createDocumentFragment();
        fr.matches.forEach(m => {
            const card = document.createElement('div');
            card.className = 'match-card';
            card.innerHTML = `
                <div class="match-header">
                    <span class="match-cat">${categoryNames[m.category] || m.category}</span>
                    <span class="match-level" style="color:${levelColors[fr.level]}">${levelNames[fr.level]}</span>
                </div>
                <div class="match-value"><code>${escHtml(m.value)}</code></div>
                <div class="match-location">行 ${m.line}, 列 ${m.column}</div>
                ${m.context ? `<div class="match-context">...${escHtml(m.context)}...</div>` : ''}
            `;
            frag.appendChild(card);
        });
        body.innerHTML = '';
        body.appendChild(frag);
    }
    modal.style.display = 'flex';
}

// === Stats Tab ===
function renderStats() {
    if (!state.summary) return;
    document.getElementById('statsEmpty').style.display = 'none';
    document.getElementById('statsContent').style.display = '';

    document.getElementById('statTotal').textContent = state.summary.total_files;
    const sensitive = Object.entries(state.summary.by_level || {})
        .filter(([k]) => levelKeyToNum(k) > 1)
        .reduce((s, [, v]) => s + v, 0);
    document.getElementById('statSensitive').textContent = sensitive;
    document.getElementById('statMatches').textContent = state.summary.total_matches;

    const highest = Object.entries(state.summary.by_level || {})
        .filter(([, v]) => v > 0)
        .map(([k]) => levelKeyToNum(k))
        .reduce((a, b) => Math.max(a, b), 0);
    document.getElementById('statHighest').textContent = highest > 0 ? levelNames[highest] : '-';

    renderLevelChart(state.summary.by_level);
    renderCategoryChart(state.summary.by_category);
}

function renderLevelChart(byLevel) {
    const container = document.getElementById('levelChart');
    container.innerHTML = '';
    // Convert backend keys like "L1-Public" to {1: count, 2: count, ...}
    const levelCounts = {};
    const levels = [1, 2, 3, 4, 5];
    levels.forEach(l => levelCounts[l] = 0);
    Object.entries(byLevel || {}).forEach(([k, v]) => {
        const n = levelKeyToNum(k);
        if (n >= 1 && n <= 5) levelCounts[n] = (levelCounts[n] || 0) + v;
    });
    const total = levels.reduce((s, l) => s + levelCounts[l], 0) || 1;
    const wrapper = document.createElement('div');
    wrapper.className = 'donut-wrapper';
    let gradientParts = [];
    let cumulative = 0;
    levels.forEach(l => {
        const count = levelCounts[l];
        const pct = (count / total) * 100;
        if (count > 0) gradientParts.push(`${levelColors[l]} ${cumulative}% ${cumulative + pct}%`);
        cumulative += pct;
    });
    const donut = document.createElement('div');
    donut.className = 'donut';
    donut.style.background = gradientParts.length > 0 ? `conic-gradient(${gradientParts.join(', ')})` : '#e0e0e0';
    const hole = document.createElement('div');
    hole.className = 'donut-hole';
    hole.textContent = total;
    donut.appendChild(hole);
    wrapper.appendChild(donut);
    const legend = document.createElement('div');
    legend.className = 'chart-legend';
    levels.forEach(l => {
        const count = levelCounts[l];
        if (count > 0) {
            const item = document.createElement('div');
            item.className = 'legend-item';
            item.innerHTML = `<span class="legend-dot" style="background:${levelColors[l]}"></span>${levelNames[l]}: ${count}`;
            legend.appendChild(item);
        }
    });
    wrapper.appendChild(legend);
    container.appendChild(wrapper);
}

function renderCategoryChart(byCategory) {
    const container = document.getElementById('categoryChart');
    container.innerHTML = '';
    const entries = Object.entries(byCategory || {}).sort((a, b) => b[1] - a[1]);
    const max = entries.length > 0 ? entries[0][1] : 1;
    const frag = document.createDocumentFragment();
    entries.forEach(([cat, count]) => {
        const row = document.createElement('div');
        row.className = 'bar-row';
        const pct = (count / max) * 100;
        row.innerHTML = `<span class="bar-label">${categoryNames[cat] || cat}</span><div class="bar-track"><div class="bar-fill" style="width:${pct}%"></div></div><span class="bar-count">${count}</span>`;
        frag.appendChild(row);
    });
    container.appendChild(frag);
}

function renderReport() {
    if (!state.report) return;
    document.getElementById('reportEmpty').style.display = 'none';
    document.getElementById('reportContent').style.display = '';
    document.getElementById('reportPath').textContent = state.report.scan_path || '-';
    document.getElementById('reportFiles').textContent = state.report.summary ? state.report.summary.total_files : 0;
    const start = state.report.started_at ? new Date(state.report.started_at * 1000) : null;
    const end = state.report.ended_at ? new Date(state.report.ended_at * 1000) : null;
    if (start && end) {
        const dur = Math.round((end - start) / 1000);
        document.getElementById('reportTime').textContent = `${start.toLocaleString()} (${dur}秒)`;
    }
}

function downloadReport(format) {
    window.location.href = `/api/export/${format}`;
}

// === Browse Directory Modal ===
async function openBrowseModal() {
    document.getElementById('browseModal').style.display = 'flex';
    const currentPath = document.getElementById('scanPath').value.trim();
    loadBrowseDir(currentPath || '');
}

async function loadBrowseDir(path) {
    const list = document.getElementById('browseList');
    list.innerHTML = '<div class="browse-loading">加载中...</div>';
    try {
        const url = '/api/browse' + (path ? '?path=' + encodeURIComponent(path) : '');
        const resp = await fetch(url);
        const data = await resp.json();
        document.getElementById('browseCurrentPath').textContent = data.current || path;
        document.getElementById('browsePathInput').value = data.current || path;
        list.innerHTML = '';
        if (data.error) { list.innerHTML = '<div class="browse-error">' + escHtml(data.error) + '</div>'; return; }
        if (!data.entries || data.entries.length === 0) { list.innerHTML = '<div class="browse-empty">无子目录</div>'; return; }
        const frag = document.createDocumentFragment();
        data.entries.forEach(entry => {
            const item = document.createElement('div');
            item.className = 'browse-item' + (entry.type === 'drive' ? ' browse-drive' : '');
            const icon = entry.type === 'drive' ? '\uD83D\uDDA5' : (entry.name === '..' ? '\u2B06' : '\uD83D\uDCC1');
            item.innerHTML = `<span class="browse-icon">${icon}</span> ${escHtml(entry.name)}`;
            item.addEventListener('click', () => { loadBrowseDir(entry.path); });
            frag.appendChild(item);
        });
        list.appendChild(frag);
    } catch (e) {
        list.innerHTML = '<div class="browse-error">加载失败: ' + escHtml(e.message) + '</div>';
    }
}

// === Utilities ===
function formatSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return (bytes / Math.pow(k, i)).toFixed(1) + ' ' + sizes[i];
}

function escHtml(s) {
    const d = document.createElement('div');
    d.textContent = s || '';
    return d.innerHTML;
}

// === Batch Toolbar ===
function updateBatchToolbar() {
    const n = state.selectedPaths.size;
    const btn = document.getElementById('batchCorrectBtn');
    btn.disabled = n === 0;
    document.getElementById('selectedCount').textContent = n > 0 ? `已选 ${n} 个文件` : '';
    document.getElementById('selectAllRows').checked = false;
}

// === Correction ===
async function submitCorrection() {
    const level = parseInt(document.getElementById('correctLevel').value);
    const note = document.getElementById('correctNote').value.trim();
    const paths = Array.from(state.selectedPaths);
    if (paths.length === 0) return;

    try {
        const resp = await fetch('/api/correct', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ paths, level, note })
        });
        const data = await resp.json();
        if (!resp.ok) { alert(data.error || '修正失败'); return; }

        // Apply corrections locally to avoid full reload
        paths.forEach(p => {
            const fr = state.results.find(r => r.path === p);
            if (fr) {
                fr.corrected_level = level;
                fr.corrected_level_name = levelNames[level] ? 'L' + level + '-' + {1:'Public',2:'Internal',3:'Confidential',4:'Secret',5:'Restricted'}[level] : '';
                fr.correction_note = note;
            }
        });
        state.selectedPaths.clear();
        updateBatchToolbar();
        rebuildFiltered();
        document.getElementById('correctModal').style.display = 'none';
    } catch (e) {
        alert('网络错误: ' + e.message);
    }
}

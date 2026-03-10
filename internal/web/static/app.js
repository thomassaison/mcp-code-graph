let cy = null;
let currentNodeId = null;

document.addEventListener('DOMContentLoaded', async () => {
    initCytoscape();
    initSearch();
    initDepthControl();
    await loadPackages();
    await loadStats();
});

// --- Package Tree ---

async function loadPackages() {
    const res = await fetch('/api/packages');
    if (!res.ok) return;
    const packages = await res.json();

    const tree = document.getElementById('package-tree');
    tree.innerHTML = '';

    for (const pkg of packages) {
        const details = document.createElement('details');
        const summary = document.createElement('summary');
        // Show short package name
        const parts = pkg.split('/');
        summary.textContent = parts.length > 2 ? parts.slice(-2).join('/') : pkg;
        summary.title = pkg;
        details.appendChild(summary);

        details.addEventListener('toggle', async () => {
            if (details.open && !details.dataset.loaded) {
                details.dataset.loaded = 'true';
                await loadPackageNodes(pkg, details);
            }
        });

        tree.appendChild(details);
    }
}

async function loadPackageNodes(pkg, container) {
    const res = await fetch('/api/packages/' + encodeURIComponent(pkg) + '/nodes');
    if (!res.ok) return;
    const nodes = await res.json();

    const div = document.createElement('div');
    div.className = 'nodes-list';

    for (const node of nodes) {
        const el = document.createElement('div');
        el.className = 'node-item';
        el.dataset.id = node.id;

        const badge = document.createElement('span');
        badge.className = 'type-badge ' + node.type;
        badge.textContent = node.type.substring(0, 3);

        const name = document.createElement('span');
        name.textContent = node.name;

        el.appendChild(badge);
        el.appendChild(name);
        el.addEventListener('click', () => selectNode(node.id));
        div.appendChild(el);
    }

    container.appendChild(div);
}

// --- Cytoscape Graph ---

function initCytoscape() {
    cy = cytoscape({
        container: document.getElementById('graph'),
        style: [
            {
                selector: 'node',
                style: {
                    'background-color': '#0f3460',
                    'label': 'data(name)',
                    'color': '#ccc',
                    'font-size': 11,
                    'text-valign': 'bottom',
                    'text-margin-y': 6,
                    'width': 40,
                    'height': 40,
                    'border-width': 2,
                    'border-color': '#0f3460'
                }
            },
            {
                selector: 'node[type = "function"]',
                style: { 'background-color': '#0f3460', 'border-color': '#53a8e2' }
            },
            {
                selector: 'node[type = "method"]',
                style: { 'background-color': '#1a3a5c', 'border-color': '#7bc4f0' }
            },
            {
                selector: 'node[type = "type"]',
                style: { 'background-color': '#2d1b4e', 'border-color': '#b48ede' }
            },
            {
                selector: 'node[type = "interface"]',
                style: { 'background-color': '#1b4332', 'border-color': '#74c69d' }
            },
            {
                selector: 'node.center',
                style: {
                    'background-color': '#e94560',
                    'border-color': '#fff',
                    'border-width': 3,
                    'width': 50,
                    'height': 50,
                    'font-size': 12,
                    'font-weight': 'bold',
                    'color': '#fff'
                }
            },
            {
                selector: 'edge',
                style: {
                    'width': 1.5,
                    'line-color': '#2a3f5f',
                    'target-arrow-color': '#2a3f5f',
                    'target-arrow-shape': 'triangle',
                    'curve-style': 'bezier',
                    'arrow-scale': 0.8
                }
            },
            {
                selector: 'edge[type = "calls"]',
                style: { 'line-color': '#53a8e2', 'target-arrow-color': '#53a8e2' }
            },
            {
                selector: 'edge[type = "implements"]',
                style: {
                    'line-color': '#74c69d',
                    'target-arrow-color': '#74c69d',
                    'line-style': 'dashed'
                }
            }
        ],
        layout: { name: 'preset' },
        minZoom: 0.3,
        maxZoom: 3
    });

    cy.on('tap', 'node', (evt) => {
        const id = evt.target.data('nodeId');
        if (id) selectNode(id);
    });
}

// --- Node Selection ---

async function selectNode(id) {
    currentNodeId = id;

    // Update tree selection
    document.querySelectorAll('.node-item.selected').forEach(el => el.classList.remove('selected'));
    const nodeEl = document.querySelector('.node-item[data-id="' + CSS.escape(id) + '"]');
    if (nodeEl) nodeEl.classList.add('selected');

    // Load node details and neighborhood in parallel
    const [nodeRes, neighborRes] = await Promise.all([
        fetch('/api/nodes/' + encodeURIComponent(id)),
        fetch('/api/nodes/' + encodeURIComponent(id) + '/neighborhood?depth=' + getDepth())
    ]);

    if (nodeRes.ok) {
        const node = await nodeRes.json();
        updateMetadata(node);
    }

    if (neighborRes.ok) {
        const data = await neighborRes.json();
        renderNeighborhood(data);
    }
}

function renderNeighborhood(data) {
    cy.elements().remove();

    // Add nodes
    for (const n of data.nodes) {
        cy.add({
            data: {
                id: 'n-' + n.id,
                nodeId: n.id,
                name: n.name,
                type: n.type
            },
            classes: n.id === data.center.id ? 'center' : ''
        });
    }

    // Add edges
    for (const e of data.edges) {
        cy.add({
            data: {
                id: 'e-' + e.from + '-' + e.to,
                source: 'n-' + e.from,
                target: 'n-' + e.to,
                type: e.type
            }
        });
    }

    // Layout
    cy.layout({
        name: 'breadthfirst',
        directed: true,
        spacingFactor: 1.2,
        roots: cy.nodes('.center'),
        animate: false
    }).run();

    cy.fit(undefined, 30);
}

// --- Metadata Panel ---

function updateMetadata(node) {
    const info = document.getElementById('node-info');

    let html = '';
    html += detailRow('Name', node.name);
    html += detailRow('Type', '<span class="type-badge ' + node.type + '">' + node.type + '</span>');
    html += detailRow('Package', node.package);

    if (node.file) {
        html += detailRow('File', node.file + ':' + node.line);
    }

    if (node.signature) {
        html += '<pre>' + escapeHtml(node.signature) + '</pre>';
    }

    if (node.docstring) {
        html += detailRow('Docs', '');
        html += '<pre>' + escapeHtml(node.docstring) + '</pre>';
    }

    if (node.summary) {
        html += detailRow('Summary', escapeHtml(node.summary));
    }

    if (node.behaviors && node.behaviors.length > 0) {
        const tags = node.behaviors.map(b => '<span class="behavior-tag">' + escapeHtml(b) + '</span>').join('');
        html += detailRow('Behaviors', '<div class="behaviors-list">' + tags + '</div>');
    }

    if (node.methods && node.methods.length > 0) {
        const methods = node.methods.map(m => escapeHtml(m.signature)).join('\n');
        html += detailRow('Methods', '');
        html += '<pre>' + methods + '</pre>';
    }

    info.innerHTML = html;
}

function detailRow(label, value) {
    return '<div class="detail-row"><span class="label">' + label + ':</span><span class="value">' + value + '</span></div>';
}

// --- Search ---

function initSearch() {
    const input = document.getElementById('search');
    const results = document.getElementById('search-results');
    let timeout = null;

    input.addEventListener('input', () => {
        clearTimeout(timeout);
        const q = input.value.trim();
        if (!q) {
            results.classList.remove('visible');
            return;
        }
        timeout = setTimeout(() => searchNodes(q), 200);
    });

    input.addEventListener('blur', () => {
        setTimeout(() => results.classList.remove('visible'), 200);
    });

    input.addEventListener('focus', () => {
        if (results.children.length > 0) results.classList.add('visible');
    });
}

async function searchNodes(query) {
    const res = await fetch('/api/search?q=' + encodeURIComponent(query));
    if (!res.ok) return;
    const nodes = await res.json();

    const results = document.getElementById('search-results');
    results.innerHTML = '';

    if (nodes.length === 0) {
        results.innerHTML = '<div class="result"><span class="result-name" style="color:#888">No results</span></div>';
        results.classList.add('visible');
        return;
    }

    for (const node of nodes) {
        const el = document.createElement('div');
        el.className = 'result';
        el.innerHTML = '<span class="result-name">' + escapeHtml(node.name) +
            '</span><span class="result-type">' + node.type + '</span>';
        el.addEventListener('mousedown', (e) => {
            e.preventDefault();
            selectNode(node.id);
            results.classList.remove('visible');
            document.getElementById('search').value = node.name;
        });
        results.appendChild(el);
    }

    results.classList.add('visible');
}

// --- Depth Control ---

function initDepthControl() {
    document.getElementById('depth').addEventListener('change', () => {
        if (currentNodeId) selectNode(currentNodeId);
    });
}

function getDepth() {
    return document.getElementById('depth').value;
}

// --- Stats ---

async function loadStats() {
    const res = await fetch('/api/stats');
    if (!res.ok) return;
    const stats = await res.json();
    document.title = 'Code Graph (' + stats.node_count + ' nodes, ' + stats.edge_count + ' edges)';
}

// --- Utilities ---

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

let cy = null;
let allData = null;

document.addEventListener('DOMContentLoaded', async () => {
    initCytoscape();
    initEdgeToggles();
    initSearch();
    await loadGraph();
});

// --- Load Full Graph ---

async function loadGraph() {
    const loading = document.getElementById('loading');
    try {
        const res = await fetch('/api/graph');
        if (!res.ok) throw new Error('Failed to load graph');
        allData = await res.json();
        renderGraph(allData);
    } catch (err) {
        loading.textContent = 'Failed to load graph: ' + err.message;
        return;
    }
    loading.classList.add('hidden');
}

function renderGraph(data) {
    cy.elements().remove();

    for (const n of data.nodes) {
        cy.add({
            data: {
                id: 'n-' + n.id,
                nodeId: n.id,
                name: n.name,
                type: n.type,
                pkg: n.package,
                file: n.file,
                line: n.line,
                signature: n.signature || '',
                docstring: n.docstring || '',
                summary: n.summary || '',
                behaviors: n.behaviors || [],
                methods: n.methods || []
            }
        });
    }

    for (const e of data.edges) {
        cy.add({
            data: {
                id: 'e-' + e.from + '-' + e.to + '-' + e.type,
                source: 'n-' + e.from,
                target: 'n-' + e.to,
                edgeType: e.type
            }
        });
    }

    applyEdgeFilters();

    cy.layout({
        name: 'cose',
        animate: false,
        nodeDimensionsIncludeLabels: true,
        nodeRepulsion: function() { return 8000; },
        idealEdgeLength: function() { return 120; },
        gravity: 0.3,
        numIter: 300
    }).run();

    cy.fit(undefined, 40);
    setupTooltips();
}

// --- Cytoscape Init ---

function initCytoscape() {
    cy = cytoscape({
        container: document.getElementById('graph'),
        style: [
            {
                selector: 'node',
                style: {
                    'label': 'data(name)',
                    'color': '#ccc',
                    'font-size': 10,
                    'text-valign': 'bottom',
                    'text-margin-y': 5,
                    'width': 30,
                    'height': 30,
                    'border-width': 2
                }
            },
            { selector: 'node[type="function"]', style: { 'background-color': '#0f3460', 'border-color': '#53a8e2' } },
            { selector: 'node[type="method"]', style: { 'background-color': '#1a3a5c', 'border-color': '#7bc4f0' } },
            { selector: 'node[type="type"]', style: { 'background-color': '#2d1b4e', 'border-color': '#b48ede' } },
            { selector: 'node[type="interface"]', style: { 'background-color': '#1b4332', 'border-color': '#74c69d' } },
            { selector: 'node[type="package"]', style: { 'background-color': '#3d2e14', 'border-color': '#e9c46a' } },
            { selector: 'node[type="file"]', style: { 'background-color': '#3d1414', 'border-color': '#e07676' } },
            {
                selector: 'node.highlighted',
                style: {
                    'border-width': 4,
                    'border-color': '#e94560',
                    'font-weight': 'bold',
                    'color': '#fff',
                    'z-index': 10
                }
            },
            {
                selector: 'node.neighbor',
                style: {
                    'border-width': 3,
                    'opacity': 1,
                    'z-index': 5
                }
            },
            {
                selector: 'node.dimmed',
                style: { 'opacity': 0.15 }
            },
            {
                selector: 'edge',
                style: {
                    'width': 1.5,
                    'line-color': '#2a3f5f',
                    'target-arrow-color': '#2a3f5f',
                    'target-arrow-shape': 'triangle',
                    'curve-style': 'bezier',
                    'arrow-scale': 0.7,
                    'opacity': 0.6
                }
            },
            { selector: 'edge[edgeType="calls"]', style: { 'line-color': '#53a8e2', 'target-arrow-color': '#53a8e2' } },
            { selector: 'edge[edgeType="implements"]', style: { 'line-color': '#74c69d', 'target-arrow-color': '#74c69d', 'line-style': 'dashed' } },
            { selector: 'edge[edgeType="uses"]', style: { 'line-color': '#888', 'target-arrow-color': '#888', 'line-style': 'dotted' } },
            { selector: 'edge[edgeType="returns"]', style: { 'line-color': '#666', 'target-arrow-color': '#666', 'width': 1 } },
            { selector: 'edge[edgeType="accepts"]', style: { 'line-color': '#666', 'target-arrow-color': '#666', 'width': 1 } },
            { selector: 'edge[edgeType="embeds"]', style: { 'line-color': '#b48ede', 'target-arrow-color': '#b48ede', 'line-style': 'dashed' } },
            { selector: 'edge[edgeType="defines"]', style: { 'line-color': '#e9c46a', 'target-arrow-color': '#e9c46a', 'width': 1 } },
            { selector: 'edge[edgeType="imports"]', style: { 'line-color': '#555', 'target-arrow-color': '#555', 'width': 1, 'line-style': 'dotted' } },
            { selector: 'edge.highlighted', style: { 'opacity': 1, 'width': 2.5, 'z-index': 10 } },
            { selector: 'edge.dimmed', style: { 'opacity': 0.05 } }
        ],
        layout: { name: 'preset' },
        minZoom: 0.1,
        maxZoom: 4,
        wheelSensitivity: 0.3
    });

    // Click node: highlight neighborhood
    cy.on('tap', 'node', (evt) => {
        highlightNode(evt.target);
    });

    // Click background: reset
    cy.on('tap', (evt) => {
        if (evt.target === cy) resetHighlight();
    });

    // Double-click: copy file:line
    cy.on('dbltap', 'node', (evt) => {
        const d = evt.target.data();
        if (d.file) {
            navigator.clipboard.writeText(d.file + ':' + d.line);
        }
    });
}

// --- Highlight ---

function highlightNode(node) {
    cy.elements().removeClass('highlighted neighbor dimmed');

    const neighborhood = node.neighborhood().add(node);
    const others = cy.elements().difference(neighborhood);

    node.addClass('highlighted');
    node.neighborhood().nodes().addClass('neighbor');
    node.connectedEdges().addClass('highlighted');
    others.addClass('dimmed');
}

function resetHighlight() {
    cy.elements().removeClass('highlighted neighbor dimmed');
}

// --- Tooltips ---

function setupTooltips() {
    if (!cy.nodes().length) return;

    cy.nodes().forEach((node) => {
        const ref = node.popperRef();
        const d = node.data();
        const content = buildTooltipContent(d);

        const tip = tippy(ref, {
            content: content,
            trigger: 'manual',
            placement: 'right',
            allowHTML: true,
            theme: 'graph',
            interactive: false,
            appendTo: document.body,
            arrow: true
        });

        node._tippy = tip;

        node.on('mouseover', () => {
            if (node._tippy) node._tippy.show();
        });

        node.on('mouseout', () => {
            if (node._tippy) node._tippy.hide();
        });
    });
}

function buildTooltipContent(d) {
    let html = '<div class="tooltip-header">' + esc(d.name) +
        '<span class="tooltip-type ' + d.type + '">' + d.type + '</span></div>';

    if (d.file) {
        html += '<div class="tooltip-file">' + esc(d.file) + ':' + d.line + '</div>';
    }

    if (d.signature) {
        html += '<div class="tooltip-sig">' + esc(d.signature) + '</div>';
    }

    if (d.summary) {
        html += '<div class="tooltip-summary">' + esc(d.summary) + '</div>';
    }

    if (d.behaviors && d.behaviors.length) {
        html += '<div class="tooltip-behaviors">';
        for (const b of d.behaviors) {
            html += '<span class="tooltip-behavior">' + esc(b) + '</span>';
        }
        html += '</div>';
    }

    return html;
}

// --- Edge Toggles ---

function initEdgeToggles() {
    document.querySelectorAll('#edge-toggles input[type="checkbox"]').forEach((cb) => {
        cb.addEventListener('change', applyEdgeFilters);
    });
}

function applyEdgeFilters() {
    const visible = new Set();
    document.querySelectorAll('#edge-toggles input:checked').forEach((cb) => {
        visible.add(cb.dataset.edge);
    });

    cy.edges().forEach((edge) => {
        const type = edge.data('edgeType');
        if (visible.has(type)) {
            edge.show();
        } else {
            edge.hide();
        }
    });
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
            resetHighlight();
            return;
        }
        timeout = setTimeout(() => searchNodes(q), 200);
    });

    input.addEventListener('blur', () => {
        setTimeout(() => results.classList.remove('visible'), 200);
    });

    input.addEventListener('focus', () => {
        if (results.children.length > 0 && input.value.trim()) {
            results.classList.add('visible');
        }
    });

    input.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            input.value = '';
            results.classList.remove('visible');
            resetHighlight();
        }
    });
}

async function searchNodes(query) {
    const res = await fetch('/api/search?q=' + encodeURIComponent(query));
    if (!res.ok) return;
    const nodes = await res.json();

    const results = document.getElementById('search-results');
    results.innerHTML = '';

    if (nodes.length === 0) {
        results.innerHTML = '<div class="result" style="color:#888">No results</div>';
        results.classList.add('visible');
        return;
    }

    for (const node of nodes) {
        const el = document.createElement('div');
        el.className = 'result';
        el.innerHTML = '<span>' + esc(node.name) + '</span><span class="result-type">' + node.type + '</span>';
        el.addEventListener('mousedown', (e) => {
            e.preventDefault();
            results.classList.remove('visible');
            focusOnNode(node.id);
        });
        results.appendChild(el);
    }

    results.classList.add('visible');
}

function focusOnNode(nodeId) {
    const node = cy.getElementById('n-' + nodeId);
    if (node.length) {
        resetHighlight();
        highlightNode(node);
        cy.animate({
            center: { eles: node },
            zoom: 2,
            duration: 400
        });
    }
}

// --- Utility ---

function esc(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

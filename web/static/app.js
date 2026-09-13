// NavisPaaS Frontend Client Logic

let state = {
  apps: [],
  offers: [],
  cluster: {},
  registry: {},
  currentDetailApp: null
};

// Initialization
document.addEventListener('DOMContentLoaded', () => {
  fetchClusterHealth();
  fetchApps();
  fetchOffers();
  fetchRegistry();
  loadBackstageTemplate();

  // Poll cluster health every 10s
  setInterval(fetchClusterHealth, 10000);
});

function switchTab(tabId) {
  document.querySelectorAll('.nav-tab').forEach(t => t.classList.remove('active'));
  document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));

  event.currentTarget.classList.add('active');
  const target = document.getElementById(`${tabId}-tab`);
  if (target) target.classList.add('active');

  if (tabId === 'apps') fetchApps();
  if (tabId === 'offers') fetchOffers();
}

async function fetchClusterHealth() {
  try {
    const res = await fetch('/api/v1/health');
    const data = await res.json();
    state.cluster = data;

    const dot = document.getElementById('statusDot');
    const text = document.getElementById('clusterStatusText');

    if (data.connected) {
      dot.className = 'status-dot connected';
      text.textContent = `Minikube Ready (${data.clusterVersion || 'v1.32'})`;
    } else {
      dot.className = 'status-dot';
      text.textContent = 'Minikube Offline (Start via `minikube start`)';
    }
  } catch (err) {
    console.error('Failed to fetch cluster health:', err);
  }
}

async function fetchApps() {
  try {
    const res = await fetch('/api/v1/apps');
    const apps = await res.json();
    state.apps = Array.isArray(apps) ? apps : [];
    renderApps();
  } catch (err) {
    console.error('Failed to fetch apps:', err);
  }
}

async function fetchOffers() {
  try {
    const res = await fetch('/api/v1/offers');
    const offers = await res.json();
    state.offers = Array.isArray(offers) ? offers : [];
    renderOffers();
  } catch (err) {
    console.error('Failed to fetch offers:', err);
  }
}

function renderApps() {
  const container = document.getElementById('appsList');
  const empty = document.getElementById('appsEmpty');

  if (state.apps.length === 0) {
    container.innerHTML = '';
    empty.style.display = 'block';
    return;
  }

  empty.style.display = 'none';
  container.innerHTML = state.apps.map(app => {
    const statusBadge = app.status === 'Running' ? 'badge-success' : 'badge-warning';
    const offersBadges = (app.linkedOffers || []).map(o => `
      <span class="badge badge-offer" title="Bound at ${new Date(o.boundAt).toLocaleTimeString()}">
        ${o.offerName}
        <button onclick="handleUnlinkOffer('${app.name}', '${o.offerId}', event)" style="background:none;border:none;color:#ef4444;cursor:pointer;margin-left:4px;font-weight:bold;">&times;</button>
      </span>
    `).join('');

    return `
      <div class="card">
        <div class="card-header">
          <div>
            <div class="card-title">${app.name}</div>
            <span style="font-size:0.8rem;color:var(--text-secondary)">ns: ${app.namespace}</span>
          </div>
          <span class="badge ${statusBadge}">${app.status}</span>
        </div>

        <div class="card-meta">
          <span>Namespace: <strong style="color:var(--accent-cyan);">${app.namespace}</strong></span>
          <span>Image: <strong>${app.image}</strong></span>
          <span>Target Port: <strong>${app.port}</strong></span>
          <span>Replicas: <strong>${app.readyCount} / ${app.replicas}</strong></span>
          ${app.accessURL ? `<span>Live URL: <a href="${app.accessURL}" target="_blank" style="color:#38bdf8;font-weight:bold;text-decoration:underline;">${app.accessURL} ↗</a></span>` : ''}
          ${app.nodePort ? `<span>NodePort: <strong>${app.nodePort}</strong></span>` : ''}
          ${app.externalIP ? `<span>Tunnel IP: <strong>${app.externalIP}</strong></span>` : ''}
          ${app.status !== 'Running' ? `<div style="background:rgba(239,68,68,0.15);border:1px solid rgba(239,68,68,0.3);border-radius:6px;padding:6px;font-size:0.8rem;color:#f87171;margin-top:6px;">⚠️ Aplicação reiniciando (${app.status}). Verifique se todas as ofertas (ex: banco de dados) foram vinculadas.</div>` : ''}
        </div>

        <div class="card-offers">
          <div class="card-offers-title">Linked Offers (${(app.linkedOffers || []).length})</div>
          <div>${offersBadges || '<span style="font-size:0.8rem;color:#64748b">None linked</span>'}</div>
        </div>

        <div class="card-actions">
          <button class="btn btn-outline btn-sm" onclick="openLinkOfferModal('${app.name}')">+ Link Offer</button>
          <button class="btn btn-outline btn-sm" onclick="viewAppDetails('${app.name}')">Logs & Pods</button>
          <button class="btn btn-danger btn-sm" onclick="handleDeleteApp('${app.name}')">Delete</button>
        </div>
      </div>
    `;
  }).join('');
}

function renderOffers() {
  const container = document.getElementById('offersList');
  container.innerHTML = state.offers.map(offer => {
    return `
      <div class="card">
        <div class="card-header">
          <div>
            <div class="card-title">${offer.name}</div>
            <span style="font-size:0.8rem;color:var(--accent-cyan);">${offer.category}</span>
          </div>
          <span class="badge ${offer.installed ? 'badge-success' : 'badge-warning'}">
            ${offer.installed ? 'Cluster Ready' : 'Not Installed'}
          </span>
        </div>

        <p style="font-size:0.875rem;color:var(--text-secondary);line-height:1.5;margin-bottom:1.25rem;">
          ${offer.description}
        </p>

        <div class="card-meta">
          <span>Version: <strong>${offer.version}</strong></span>
          <span>Auto-Binding Envs: <strong>Supported</strong></span>
        </div>

        <div class="card-actions">
          ${!offer.installed ? `
            <button class="btn btn-primary btn-sm" onclick="handleInstallOffer('${offer.id}')">
              ⚡ Install to Minikube
            </button>
          ` : `
            <button class="btn btn-outline btn-sm" disabled style="opacity:0.6;cursor:default;">
              ✓ Installed
            </button>
          `}
          ${offer.id === 'monitoring' && offer.installed ? `
            <a href="http://192.168.49.2:30080" target="_blank" class="btn btn-outline btn-sm" style="text-decoration:none;color:#38bdf8;">
              Open Grafana (192.168.49.2:30080) ↗
            </a>
          ` : ''}
        </div>
      </div>
    `;
  }).join('');
}

async function fetchRegistry() {
  try {
    const res = await fetch('/api/v1/registry');
    const data = await res.json();
    state.registry = data;
  } catch (err) {
    console.error('Failed to fetch registry config:', err);
  }
}

function openRegistryModal() {
  const reg = state.registry || {};
  document.getElementById('regEnabled').checked = !!reg.enabled;
  document.getElementById('regServer').value = reg.server || 'ghcr.io';
  document.getElementById('regUsername').value = reg.username || '';
  document.getElementById('regPassword').value = '';
  document.getElementById('regDefaultPrefix').value = reg.defaultPrefix || '';
  document.getElementById('registryModal').classList.add('open');
}

async function handleSaveRegistry(e) {
  e.preventDefault();
  const enabled = document.getElementById('regEnabled').checked;
  const server = document.getElementById('regServer').value.trim();
  const username = document.getElementById('regUsername').value.trim();
  const password = document.getElementById('regPassword').value.trim();
  const email = document.getElementById('regEmail').value.trim();
  const defaultPrefix = document.getElementById('regDefaultPrefix').value.trim();

  try {
    const res = await fetch('/api/v1/registry', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled, server, username, password, email, defaultPrefix })
    });

    if (!res.ok) {
      const err = await res.json();
      alert('Failed to save registry: ' + (err.error || res.statusText));
      return;
    }

    const data = await res.json();
    state.registry = data.registry;
    alert('Registry settings saved and synchronized to Kubernetes!');
    closeModal('registryModal');
  } catch (err) {
    alert('Error saving registry: ' + err.message);
  }
}

// Modal Handlers
function openDeployModal() {
  // If a default prefix is configured, show a helper hint in placeholder
  if (state.registry && state.registry.enabled && state.registry.defaultPrefix) {
    document.getElementById('appImage').placeholder = `e.g. order-service:v1 (auto-prepended with ${state.registry.defaultPrefix}/)`;
  }
  document.getElementById('deployModal').classList.add('open');
}

function closeModal(modalId) {
  document.getElementById(modalId).classList.remove('open');
}

async function handleDeployApp(e) {
  e.preventDefault();
  const name = document.getElementById('appName').value.trim();
  const namespace = document.getElementById('appNamespace').value.trim();
  const image = document.getElementById('appImage').value.trim();
  const port = parseInt(document.getElementById('appPort').value, 10);
  const replicas = parseInt(document.getElementById('appReplicas').value, 10);

  const offers = [];
  if (document.getElementById('offerCheckMonitoring').checked) offers.push('monitoring');
  if (document.getElementById('offerCheckKafka').checked) offers.push('kafka');

  try {
    const payload = { name, image, port, replicas, offers };
    if (namespace) payload.namespace = namespace;

    const res = await fetch('/api/v1/apps', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });

    if (!res.ok) {
      const err = await res.json();
      alert('Failed to deploy: ' + (err.error || res.statusText));
      return;
    }

    closeModal('deployModal');
    document.getElementById('deployForm').reset();
    fetchApps();
  } catch (err) {
    alert('Error deploying app: ' + err.message);
  }
}

function openLinkOfferModal(appName) {
  document.getElementById('linkAppTargetName').value = appName;
  document.getElementById('linkOfferTitle').textContent = `Link Offer to '${appName}'`;

  const select = document.getElementById('linkOfferSelect');
  select.innerHTML = state.offers.map(o => `<option value="${o.id}">${o.name} (${o.category})</option>`).join('');

  renderOfferParameters();
  document.getElementById('linkOfferModal').classList.add('open');
}

function renderOfferParameters() {
  const offerId = document.getElementById('linkOfferSelect').value;
  const offer = state.offers.find(o => o.id === offerId);
  const container = document.getElementById('offerDynamicParams');

  if (!offer || !offer.parameters || offer.parameters.length === 0) {
    container.innerHTML = '<p style="color:var(--text-secondary);font-size:0.85rem;">No additional parameters required.</p>';
    return;
  }

  container.innerHTML = offer.parameters.map(param => `
    <div class="form-group">
      <label>${param.label} ${param.required ? '<span style="color:#ef4444">*</span>' : ''}</label>
      <input type="${param.type === 'number' ? 'number' : 'text'}" 
             name="param_${param.key}" 
             class="form-control" 
             value="${param.default || ''}" 
             ${param.required ? 'required' : ''}>
      <small style="color:#64748b;font-size:0.75rem;">${param.description}</small>
    </div>
  `).join('');
}

async function handleLinkOfferSubmit(e) {
  e.preventDefault();
  const appName = document.getElementById('linkAppTargetName').value;
  const offerId = document.getElementById('linkOfferSelect').value;
  const offer = state.offers.find(o => o.id === offerId);

  const parameters = {};
  if (offer && offer.parameters) {
    offer.parameters.forEach(p => {
      const input = document.querySelector(`[name="param_${p.key}"]`);
      if (input) parameters[p.key] = input.value;
    });
  }

  try {
    const res = await fetch(`/api/v1/apps/${appName}/links`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ offerId, parameters })
    });

    if (!res.ok) {
      const err = await res.json();
      alert('Failed to link offer: ' + (err.error || res.statusText));
      return;
    }

    closeModal('linkOfferModal');
    fetchApps();
  } catch (err) {
    alert('Error linking offer: ' + err.message);
  }
}

async function handleUnlinkOffer(appName, offerId, e) {
  e.stopPropagation();
  if (!confirm(`Are you sure you want to unlink '${offerId}' from '${appName}'?`)) return;

  try {
    const res = await fetch(`/api/v1/apps/${appName}/links/${offerId}`, { method: 'DELETE' });
    if (res.ok) fetchApps();
  } catch (err) {
    alert('Failed to unlink: ' + err.message);
  }
}

async function handleDeleteApp(appName) {
  if (!confirm(`Are you sure you want to delete application '${appName}'?`)) return;

  try {
    const res = await fetch(`/api/v1/apps/${appName}`, { method: 'DELETE' });
    if (res.ok) fetchApps();
  } catch (err) {
    alert('Failed to delete: ' + err.message);
  }
}

async function handleInstallOffer(offerId) {
  if (!confirm(`Install '${offerId}' cluster components onto Minikube?`)) return;

  try {
    const res = await fetch(`/api/v1/offers/${offerId}/install`, { method: 'POST' });
    const data = await res.json();
    alert(data.message || 'Installed successfully');
    fetchOffers();
  } catch (err) {
    alert('Installation failed: ' + err.message);
  }
}

async function viewAppDetails(appName) {
  state.currentDetailApp = appName;
  document.getElementById('detailAppTitle').textContent = `App Details: ${appName}`;
  document.getElementById('detailPodsList').innerHTML = 'Loading pods...';
  document.getElementById('detailEnvVars').textContent = 'Loading envs...';
  document.getElementById('detailLogs').textContent = 'Fetching logs...';

  document.getElementById('appDetailsModal').classList.add('open');

  try {
    const res = await fetch(`/api/v1/apps/${appName}`);
    const details = await res.json();

    // Render pods
    const pods = details.pods || [];
    if (pods.length === 0) {
      document.getElementById('detailPodsList').innerHTML = '<span style="color:#64748b">No active pods found.</span>';
    } else {
      document.getElementById('detailPodsList').innerHTML = pods.map(p => `
        <div style="background:var(--bg-primary);padding:0.6rem;border-radius:6px;margin-bottom:0.4rem;display:flex;justify-content:space-between;font-size:0.85rem;">
          <span><strong>${p.name}</strong> (${p.status})</span>
          <span style="color:var(--text-secondary)">IP: ${p.ip || 'Pending'} | Restarts: ${p.restarts}</span>
        </div>
      `).join('');
    }

    // Render env vars
    const envs = details.application.envVars || {};
    document.getElementById('detailEnvVars').textContent = Object.keys(envs).length > 0 
      ? JSON.stringify(envs, null, 2) 
      : 'No custom environment variables injected.';

    // Fetch Logs
    refreshCurrentLogs();
  } catch (err) {
    console.error(err);
  }
}

async function refreshCurrentLogs() {
  if (!state.currentDetailApp) return;
  try {
    const res = await fetch(`/api/v1/apps/${state.currentDetailApp}/logs?lines=60`);
    const data = await res.json();
    document.getElementById('detailLogs').textContent = data.logs || 'No logs available.';
  } catch (err) {
    document.getElementById('detailLogs').textContent = 'Error loading logs: ' + err.message;
  }
}

function loadBackstageTemplate() {
  const templateYaml = `apiVersion: scaffolder.backstage.io/v1beta3
kind: Template
metadata:
  name: navispaas-deploy-template
  title: Deploy Service via NavisPaaS
  description: Scaffold a cloud-native service with Kafka messaging and Prometheus/Grafana observability.
spec:
  owner: platform-team
  type: service
  parameters:
    - title: Service Details
      required: [name, image, port]
      properties:
        name:
          title: Application Name
          type: string
          description: Unique service name
        image:
          title: Docker Image
          type: string
          default: "nginx:alpine"
        port:
          title: Container Port
          type: number
          default: 80
    - title: Offers & Addons
      properties:
        enableKafka:
          title: Attach Apache Kafka
          type: boolean
          default: true
        enableMonitoring:
          title: Attach Observability Stack (Prometheus + Grafana)
          type: boolean
          default: true
  steps:
    - id: deploy-to-navis
      name: Trigger NavisPaaS Engine
      action: http:backstage:request
      input:
        method: POST
        url: http://navispaas.internal:8080/api/v1/apps
        body:
          name: \${{ parameters.name }}
          image: \${{ parameters.image }}
          port: \${{ parameters.port }}
          offers:
            - \${{ parameters.enableKafka ? 'kafka' : '' }}
            - \${{ parameters.enableMonitoring ? 'monitoring' : '' }}`;

  const el = document.getElementById('backstageTemplateCode');
  if (el) el.textContent = templateYaml;
}

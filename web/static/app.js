// Evolved NavisPaaS Alpine.js Application Logic
function navisApp() {
  return {
    activeTab: 'apps',
    apps: [],
    offers: [],
    cluster: { connected: false },
    registry: {},
    monitoringStatus: { prometheus: false, grafana: false, otel: false },
    clusterContexts: [],
    
    // Modal visibilities
    modals: {
      deploy: false,
      cluster: false,
      customOffer: false,
      link: false,
      details: false,
      registry: false,
    },

    // Forms states
    registryForm: {
      enabled: false,
      server: 'ghcr.io',
      username: '',
      password: '',
      email: '',
      defaultPrefix: '',
    },

    deployForm: {
      name: '',
      namespace: '',
      image: '',
      port: 80,
      replicas: 1,
      linkPostgres: false,
      linkRedis: false,
      linkKafka: false,
      linkMonitoring: false,
    },

    clusterForm: {
      contextName: '',
      externalName: '',
      externalKubeconfig: '',
    },

    customOfferForm: {
      id: '',
      name: '',
      category: 'Database',
      envPrefix: '',
      image: '',
      port: 80,
      description: '',
    },

    linkForm: {
      appName: '',
      appNamespace: '',
      offerId: '',
      params: {},
    },

    appDetails: {
      name: '',
      namespace: '',
      pods: [],
      envVars: {},
      logs: 'Loading logs...',
    },

    backstageTemplate: '',

    init() {
      this.fetchClusterHealth();
      this.fetchApps();
      this.fetchOffers();
      this.fetchRegistry();
      this.loadBackstageTemplate();

      // Poll cluster health & apps every 8 seconds
      setInterval(() => {
        this.fetchClusterHealth();
        this.fetchApps();
        this.fetchOffers();
      }, 8000);
    },

    async fetchClusterHealth() {
      try {
        const res = await fetch('/api/v1/health');
        if (res.ok) {
          this.cluster = await res.json();
        } else {
          this.cluster = { connected: false };
        }
      } catch (err) {
        console.error('Failed to fetch cluster health:', err);
      }
    },

    async fetchApps() {
      try {
        const res = await fetch('/api/v1/apps');
        if (res.ok) {
          const data = await res.json();
          this.apps = Array.isArray(data) ? data : [];
        }
      } catch (err) {
        console.error('Failed to fetch apps:', err);
      }
    },

    async fetchOffers() {
      try {
        const res = await fetch('/api/v1/offers');
        if (res.ok) {
          const data = await res.json();
          this.offers = Array.isArray(data) ? data : [];

          // Fetch monitoring status if available
          const hasMonitoring = this.offers.find(o => o.id === 'monitoring' && o.installed);
          if (hasMonitoring) {
            this.fetchMonitoringStatus();
          }
        }
      } catch (err) {
        console.error('Failed to fetch offers:', err);
      }
    },

    async fetchMonitoringStatus() {
      try {
        const res = await fetch('/api/v1/offers/monitoring/status');
        if (res.ok) {
          this.monitoringStatus = await res.json();
        }
      } catch (err) {
        console.error('Failed to fetch monitoring status:', err);
      }
    },

    async fetchRegistry() {
      try {
        const res = await fetch('/api/v1/registry');
        if (res.ok) {
          this.registry = await res.json();
        }
      } catch (err) {
        console.error('Failed to fetch registry config:', err);
      }
    },

    // Modal Control Helpers
    openDeployModal() {
      this.deployForm = {
        name: '',
        namespace: '',
        image: '',
        port: 80,
        replicas: 1,
        linkPostgres: false,
        linkRedis: false,
        linkKafka: false,
        linkMonitoring: false,
      };
      if (this.registry && this.registry.enabled && this.registry.defaultPrefix) {
        this.deployForm.image = this.registry.defaultPrefix + '/';
      }
      this.modals.deploy = true;
    },

    async openClusterModal() {
      this.clusterForm = {
        contextName: this.cluster.context || '',
        externalName: '',
        externalKubeconfig: '',
      };
      this.modals.cluster = true;
      try {
        const res = await fetch('/api/v1/cluster/contexts');
        if (res.ok) {
          const data = await res.json();
          this.clusterContexts = data.contexts || [];
          if (!this.clusterForm.contextName) {
            this.clusterForm.contextName = data.current || '';
          }
        }
      } catch (err) {
        console.error('Failed to fetch contexts:', err);
      }
    },

    openCustomOfferModal() {
      this.customOfferForm = {
        id: '',
        name: '',
        category: 'Database',
        envPrefix: '',
        image: '',
        port: 80,
        description: '',
      };
      this.modals.customOffer = true;
    },

    openLinkOfferModal(appName, appNamespace) {
      this.linkForm = {
        appName: appName,
        appNamespace: appNamespace,
        offerId: this.offers.length > 0 ? this.offers[0].id : '',
        params: {},
      };
      this.modals.link = true;
    },

    closeModal(name) {
      this.modals[name] = false;
    },

    // Deployment logic
    async handleDeployApp() {
      const offersList = [];
      if (this.deployForm.linkPostgres) offersList.push('postgresql');
      if (this.deployForm.linkRedis) offersList.push('redis');
      if (this.deployForm.linkKafka) offersList.push('kafka');
      if (this.deployForm.linkMonitoring) offersList.push('monitoring');

      const payload = {
        name: this.deployForm.name.trim(),
        namespace: this.deployForm.namespace.trim() || this.deployForm.name.trim(),
        image: this.deployForm.image.trim(),
        port: this.deployForm.port,
        replicas: this.deployForm.replicas,
        offers: offersList,
      };

      try {
        const res = await fetch('/api/v1/apps', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });

        const data = await res.json();
        if (!res.ok) {
          alert('Deployment failed: ' + (data.detail || data.error || res.statusText));
          return;
        }

        this.closeModal('deploy');
        this.fetchApps();
      } catch (err) {
        alert('Error deploying application: ' + err.message);
      }
    },

    // Offer Linking & Custom Params
    getSelectedOfferParameters() {
      const selected = this.offers.find(o => o.id === this.linkForm.offerId);
      return (selected && selected.parameters) ? selected.parameters : [];
    },

    async handleLinkOfferSubmit() {
      const appName = this.linkForm.appName;
      const appNamespace = this.linkForm.appNamespace;
      const offerId = this.linkForm.offerId;
      const parameters = {};

      const offer = this.offers.find(o => o.id === offerId);
      if (offer && offer.parameters) {
        offer.parameters.forEach(p => {
          const val = this.linkForm.params[p.key];
          if (val !== undefined && val !== '') {
            parameters[p.key] = String(val);
          } else if (p.default) {
            parameters[p.key] = p.default;
          }
        });
      }

      try {
        const url = appNamespace 
          ? `/api/v1/apps/${appName}/links?namespace=${encodeURIComponent(appNamespace)}`
          : `/api/v1/apps/${appName}/links`;

        const res = await fetch(url, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ offerId, parameters })
        });

        const data = await res.json();
        if (!res.ok) {
          alert('Linking failed: ' + (data.detail || data.error || res.statusText));
          return;
        }

        this.closeModal('link');
        this.fetchApps();
      } catch (err) {
        alert('Error linking offer: ' + err.message);
      }
    },

    async handleUnlinkOffer(appName, appNamespace, offerId) {
      if (!confirm(`Are you sure you want to unlink '${offerId}' from '${appName}'?`)) return;

      try {
        const url = appNamespace 
          ? `/api/v1/apps/${appName}/links/${offerId}?namespace=${encodeURIComponent(appNamespace)}`
          : `/api/v1/apps/${appName}/links/${offerId}`;

        const res = await fetch(url, { method: 'DELETE' });
        const data = await res.json();
        if (!res.ok) {
          alert('Unlinking failed: ' + (data.detail || data.error || res.statusText));
          return;
        }

        this.fetchApps();
      } catch (err) {
        alert('Error unlinking offer: ' + err.message);
      }
    },

    // Offer Cluster Installation
    async handleInstallOffer(offerId) {
      const provider = this.cluster.provider || 'kubernetes';
      if (!confirm(`Install '${offerId}' cluster components onto ${provider}?`)) return;

      try {
        const res = await fetch(`/api/v1/offers/${offerId}/install`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({})
        });
        const data = await res.json();
        if (!res.ok) {
          alert('Installation failed: ' + (data.detail || data.error || res.statusText));
          return;
        }
        alert(data.message || 'Installed successfully');
        this.fetchOffers();
      } catch (err) {
        alert('Error initiating install: ' + err.message);
      }
    },

    async handleInstallMonitoring() {
      const prometheus = !!this.monitoringStatus.prometheus;
      const grafana = !!this.monitoringStatus.grafana;
      const otel = !!this.monitoringStatus.otel;

      if (!prometheus && !grafana && !otel) {
        alert('Please select at least one component to install.');
        return;
      }

      const provider = this.cluster.provider || 'kubernetes';
      if (!confirm(`Install selected monitoring components onto ${provider}?`)) return;

      try {
        const res = await fetch('/api/v1/offers/monitoring/install', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ prometheus, grafana, otel })
        });
        const data = await res.json();
        if (!res.ok) {
          alert('Installation failed: ' + (data.detail || data.error || res.statusText));
          return;
        }
        alert(data.message || 'Installed successfully');
        this.fetchOffers();
      } catch (err) {
        alert('Error starting monitoring install: ' + err.message);
      }
    },

    // Cluster contexts & kubeconfig paste
    async handleSwitchContext() {
      const contextName = this.clusterForm.contextName;
      if (!contextName) return;

      try {
        const res = await fetch('/api/v1/cluster/context', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ contextName })
        });

        const data = await res.json();
        if (!res.ok) {
          alert('Failed to switch context: ' + (data.detail || data.error || res.statusText));
          return;
        }

        alert(`Switched active context to '${contextName}'!`);
        this.closeModal('cluster');
        this.fetchClusterHealth();
        this.fetchApps();
        this.fetchOffers();
      } catch (err) {
        alert('Error switching context: ' + err.message);
      }
    },

    async handleSaveKubeconfig() {
      const contextName = this.clusterForm.externalName.trim();
      const kubeconfig = this.clusterForm.externalKubeconfig.trim();

      try {
        const res = await fetch('/api/v1/cluster/kubeconfig', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ kubeconfig, contextName })
        });

        const data = await res.json();
        if (!res.ok) {
          alert('Failed to connect to cluster: ' + (data.detail || data.error || res.statusText));
          return;
        }

        alert('Successfully connected to external cluster!');
        this.closeModal('cluster');
        this.fetchClusterHealth();
        this.fetchApps();
        this.fetchOffers();
      } catch (err) {
        alert('Error connecting: ' + err.message);
      }
    },

    // Custom offer creation
    async handleCreateCustomOffer() {
      const id = this.customOfferForm.id.trim().toLowerCase().replace(/[^a-z0-9-]/g, '');
      const name = this.customOfferForm.name.trim();
      const category = this.customOfferForm.category;
      const envPrefix = this.customOfferForm.envPrefix.trim().toUpperCase();
      const image = this.customOfferForm.image.trim();
      const port = this.customOfferForm.port;
      const description = this.customOfferForm.description.trim();

      const payload = { id, name, category, envPrefix, image, port, description, version: '1.0' };

      try {
        const res = await fetch('/api/v1/offers/custom', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });

        const data = await res.json();
        if (!res.ok) {
          alert('Failed to create offer: ' + (data.detail || data.error || res.statusText));
          return;
        }

        alert(`Custom offer '${name}' created and registered!`);
        this.closeModal('customOffer');
        this.fetchOffers();
      } catch (err) {
        alert('Error creating offer: ' + err.message);
      }
    },

    async handleDeleteCustomOffer(offerId) {
      if (!confirm(`Are you sure you want to delete custom offer '${offerId}'?`)) return;

      try {
        const res = await fetch(`/api/v1/offers/custom/${offerId}`, { method: 'DELETE' });
        const data = await res.json();
        if (!res.ok) {
          alert('Delete failed: ' + (data.detail || data.error || res.statusText));
          return;
        }
        this.fetchOffers();
      } catch (err) {
        alert('Error deleting custom offer: ' + err.message);
      }
    },

    // App logs & detailed info modal
    async viewAppDetails(appName, appNamespace) {
      this.appDetails = {
        name: appName,
        namespace: appNamespace,
        pods: [],
        envVars: {},
        logs: 'Loading logs...',
      };
      this.modals.details = true;
      this.refreshLogs();

      try {
        const url = appNamespace 
          ? `/api/v1/apps/${appName}?namespace=${encodeURIComponent(appNamespace)}`
          : `/api/v1/apps/${appName}`;
        const res = await fetch(url);
        if (res.ok) {
          const details = await res.json();
          this.appDetails.pods = details.pods || [];
          this.appDetails.envVars = details.application?.envVars || {};
        }
      } catch (err) {
        console.error('Failed to load app details:', err);
      }
    },

    async refreshLogs() {
      if (!this.appDetails.name) return;
      this.appDetails.logs = 'Fetching latest logs...';
      try {
        const url = this.appDetails.namespace 
          ? `/api/v1/apps/${this.appDetails.name}/logs?lines=60&namespace=${encodeURIComponent(this.appDetails.namespace)}`
          : `/api/v1/apps/${this.appDetails.name}/logs?lines=60`;
        const res = await fetch(url);
        if (res.ok) {
          const data = await res.json();
          this.appDetails.logs = data.logs || 'No logs available.';
        } else {
          this.appDetails.logs = 'Failed to fetch logs.';
        }
      } catch (err) {
        this.appDetails.logs = 'Error fetching logs: ' + err.message;
      }
    },

    async handleDeleteApp(appName, appNamespace) {
      if (!confirm(`Are you sure you want to delete application '${appName}'?`)) return;

      try {
        const url = appNamespace 
          ? `/api/v1/apps/${appName}?namespace=${encodeURIComponent(appNamespace)}`
          : `/api/v1/apps/${appName}`;

        const res = await fetch(url, { method: 'DELETE' });
        const data = await res.json();
        if (!res.ok) {
          alert('Delete failed: ' + (data.detail || data.error || res.statusText));
          return;
        }

        this.fetchApps();
      } catch (err) {
        alert('Error deleting application: ' + err.message);
      }
    },

    // Save Registry config
    async handleSaveRegistry() {
      try {
        const res = await fetch('/api/v1/registry', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(this.registryForm)
        });

        const data = await res.json();
        if (!res.ok) {
          alert('Failed to save registry: ' + (data.detail || data.error || res.statusText));
          return;
        }

        this.registry = data.registry || {};
        alert('Registry settings saved and synchronized to cluster!');
        this.closeModal('registry');
      } catch (err) {
        alert('Error saving registry: ' + err.message);
      }
    },

    openRegistryModal() {
      const reg = this.registry || {};
      this.registryForm = {
        enabled: !!reg.enabled,
        server: reg.server || 'ghcr.io',
        username: reg.username || '',
        password: '',
        email: reg.email || '',
        defaultPrefix: reg.defaultPrefix || '',
      };
      this.modals.registry = true;
    },

    loadBackstageTemplate() {
      this.backstageTemplate = `apiVersion: scaffolder.backstage.io/v1beta3
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
    }
  };
}

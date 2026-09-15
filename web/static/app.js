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

     // Notification system (non-blocking replacement for alert())
     notification: {
       show: false,
       message: '',
       type: 'info', // 'success', 'error', 'warning', 'info'
     },
     _notificationTimeout: null,

      showNotification(message, type = 'info') {
        if (this._notificationTimeout) {
          clearTimeout(this._notificationTimeout);
        }
        this.notification.show = true;
        this.notification.message = String(message);
        this.notification.type = type;
        this._notificationTimeout = setTimeout(() => {
          this.notification.show = false;
        }, 4000);
      },

     hideNotification() {
       if (this._notificationTimeout) {
         clearTimeout(this._notificationTimeout);
         this._notificationTimeout = null;
       }
       this.notification.show = false;
     },

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
        this.cluster = { connected: false };
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
          let parsedOffers = Array.isArray(data) ? data : [];
          
          // Sort offers by Category then by Name then by ID to prevent UI shuffling
          parsedOffers.sort((a, b) => {
            if (a.category < b.category) return -1;
            if (a.category > b.category) return 1;
            if (a.name < b.name) return -1;
            if (a.name > b.name) return 1;
            if (a.id < b.id) return -1;
            if (a.id > b.id) return 1;
            return 0;
          });

          this.offers = parsedOffers;

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
          this.showNotification('Deployment failed: ' + (data.detail || data.error || res.statusText), 'error');
          return;
        }

        this.showNotification('Application deployed successfully!', 'success');
        this.closeModal('deploy');
        this.fetchApps();
      } catch (err) {
        this.showNotification('Error deploying application: ' + err.message, 'error');
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
          this.showNotification('Linking failed: ' + (data.detail || data.error || res.statusText), 'error');
          return;
        }

        this.showNotification('Offer linked successfully!', 'success');
        this.closeModal('link');
        this.fetchApps();
      } catch (err) {
        this.showNotification('Error linking offer: ' + err.message, 'error');
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
          this.showNotification('Unlinking failed: ' + (data.detail || data.error || res.statusText), 'error');
          return;
        }

        this.showNotification('Offer unlinked successfully!', 'success');
        this.fetchApps();
      } catch (err) {
        this.showNotification('Error unlinking offer: ' + err.message, 'error');
      }
    },

    // Offer Cluster Installation
    async handleInstallOffer(offerId) {
      const provider = this.cluster.provider || 'kubernetes';
      
      // Se confirm estiver bloqueado pelo browser, window.confirm pode retornar false silencioso
      if (!window.confirm(`Install '${offerId}' cluster components onto ${provider}?`)) {
        return;
      }

      try {
        const res = await fetch(`/api/v1/offers/${offerId}/install`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({})
        });

        // Parse seguro para suportar 204 No Content ou texto puro
        let data = {};
        const text = await res.text();
        if (text) {
          try { data = JSON.parse(text); } catch (_) { data = { message: text }; }
        }

        if (!res.ok) {
          this.showNotification('Installation failed: ' + (data.detail || data.error || res.statusText), 'error');
          return;
        }

        this.showNotification(data.message || 'Installed successfully', 'success');
        await this.fetchOffers();
      } catch (err) {
        console.error('Error initiating install:', err);
        this.showNotification('Error initiating install: ' + err.message, 'error');
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
          this.showNotification('Installation failed: ' + (data.detail || data.error || res.statusText), 'error');
          return;
        }
        this.showNotification(data.message || 'Installed successfully', 'success');
        this.fetchOffers();
      } catch (err) {
        this.showNotification('Error starting monitoring install: ' + err.message, 'error');
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
          this.showNotification('Failed to switch context: ' + (data.detail || data.error || res.statusText), 'error');
          return;
        }

        this.showNotification(`Switched active context to '${contextName}'!`, 'success');
        this.closeModal('cluster');
        this.fetchClusterHealth();
        this.fetchApps();
        this.fetchOffers();
      } catch (err) {
        this.showNotification('Error switching context: ' + err.message, 'error');
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
          this.showNotification('Failed to connect to cluster: ' + (data.detail || data.error || res.statusText), 'error');
          return;
        }

        this.showNotification('Successfully connected to external cluster!', 'success');
        this.closeModal('cluster');
        this.fetchClusterHealth();
        this.fetchApps();
        this.fetchOffers();
      } catch (err) {
        this.showNotification('Error connecting: ' + err.message, 'error');
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
          this.showNotification('Failed to create offer: ' + (data.detail || data.error || res.statusText), 'error');
          return;
        }

        this.showNotification(`Custom offer '${name}' created and registered!`, 'success');
        this.closeModal('customOffer');
        this.fetchOffers();
      } catch (err) {
        this.showNotification('Error creating offer: ' + err.message, 'error');
      }
    },

    async handleDeleteCustomOffer(offerId) {
      if (!confirm(`Are you sure you want to delete custom offer '${offerId}'?`)) return;

      try {
        const res = await fetch(`/api/v1/offers/custom/${offerId}`, { method: 'DELETE' });
        const data = await res.json();
        if (!res.ok) {
          this.showNotification('Delete failed: ' + (data.detail || data.error || res.statusText), 'error');
          return;
        }
        this.showNotification(`Custom offer '${offerId}' deleted successfully!`, 'success');
        this.fetchOffers();
      } catch (err) {
        this.showNotification('Error deleting custom offer: ' + err.message, 'error');
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
      if (!window.confirm(`Are you sure you want to delete application '${appName}'?`)) {
        return;
      }

      try {
        const url = appNamespace 
          ? `/api/v1/apps/${appName}?namespace=${encodeURIComponent(appNamespace)}`
          : `/api/v1/apps/${appName}`;

        const res = await fetch(url, { method: 'DELETE' });

        // Suporta 204 No Content sem estourar SyntaxError
        let data = {};
        const text = await res.text();
        if (text) {
          try { data = JSON.parse(text); } catch (_) { data = { message: text }; }
        }

        if (!res.ok) {
          this.showNotification('Delete failed: ' + (data.detail || data.error || res.statusText), 'error');
          return;
        }

        this.showNotification(`Application '${appName}' deleted successfully!`, 'success');
        await this.fetchApps();
      } catch (err) {
        console.error('Error deleting application:', err);
        this.showNotification('Error deleting application: ' + err.message, 'error');
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

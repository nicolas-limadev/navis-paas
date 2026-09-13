# NavisPaaS 🚢⚡

> **Developer-Centric Kubernetes PaaS Engine**  
> Deploy microservices effortlessly and attach pre-integrated infrastructure offers (Kafka, Prometheus & Grafana Monitoring, Redis, Postgres) with automatic service bindings and zero boilerplate.

---

## 🌟 Visão Geral

O **NavisPaaS** foi concebido para resolver o atrito entre o desenvolvedor e a complexidade do Kubernetes. 

Em vez de criar dezenas de manifestos YAML (`Deployment`, `Service`, `ConfigMap`, `ServiceMonitor`, `KafkaTopic`), o desenvolvedor informa sua imagem e porta, seleciona as **ofertas** desejadas e a plataforma cuida do provisionamento e da injeção das configurações via variáveis de ambiente e annotations.

```
┌────────────────────────────────────────────────────────┐
│                   Spotify Backstage                    │
│            (Developer Portal / Scaffolder)             │
│   - Catálogo de Serviços                               │
│   - Templates Self-Service com 1 clique                │
└──────────────────────────┬─────────────────────────────┘
                           │ REST API / OpenAPI 3.0
┌──────────────────────────▼─────────────────────────────┐
│                 NavisPaaS Core Engine                  │
│                     (Backend em Go)                    │
│  - REST API (Gin + OpenAPI Spec)                       │
│  - Web Dashboard SPA (Dark Mode)                       │
│  - Catalog & Service Binding Engine                    │
└──────────────────────────┬─────────────────────────────┘
                           │ client-go / Kube API
┌──────────────────────────▼─────────────────────────────┐
│                   Cluster Minikube                     │
│  • Workloads dos Devs (navis-apps)                     │
│  • Oferta Kafka: Apache Kafka (namespace: kafka)       │
│  • Oferta Observability: Prometheus + Grafana (30080)  │
└────────────────────────────────────────────────────────┘
```

---

## 🧩 Ofertas Pré-Integradas (Addons)

| Oferta | Categoria | O que o NavisPaaS faz ao linkar |
| :--- | :--- | :--- |
| **Apache Kafka** | *Messaging* | Provisiona o tópico, cria ConfigMap de binding e injeta `KAFKA_BOOTSTRAP_SERVERS`, `KAFKA_TOPIC`, `KAFKA_CONSUMER_GROUP` e `KAFKA_CLIENT_ID` no pod. |
| **Observability Stack** | *Observability* | Configura o Prometheus para raspar o endpoint `/metrics`, injeta endpoints do OpenTelemetry Collector e disponibiliza o dashboard no Grafana (`:30080`). |
| **Redis Cache** | *Database* | Provisiona instância Redis e injeta `REDIS_HOST`, `REDIS_PORT`, `REDIS_URL`. |
| **PostgreSQL DB** | *Database* | Provisiona instância PostgreSQL e injeta `DB_HOST`, `DB_PORT`, `DATABASE_URL`. |

---

## 🚀 Como Executar

### 1. Iniciar o Minikube
```bash
minikube start --driver=docker --cpus=2 --memory=4096
```

### 2. Compilar e Rodar o NavisPaaS
```bash
cd /home/nicolas/projects-github/navispaas
make run
```

Acesse o **Dashboard Web**:
👉 **http://localhost:8080**

Documentação OpenAPI:
👉 **http://localhost:8080/api/v1/openapi.json**

---

## 🎭 Integração com Spotify Backstage

O NavisPaaS foi desenhado para ser acoplado diretamente ao Backstage:

1. **Catálogo de Componentes**: O arquivo `backstage/catalog-info.yaml` registra a API e o serviço do NavisPaaS no catálogo do Backstage.
2. **Software Template (Scaffolder)**: O arquivo `backstage/template-app.yaml` define o formulário no Backstage para os devs criarem novas aplicações escolhendo as caixas de seleção do **Kafka** e da **Monitoração**.

---

## 📡 Referência Rápida da API REST

| Método | Endpoint | Descrição |
| :--- | :--- | :--- |
| `GET` | `/api/v1/health` | Status de conectividade com o Minikube / K8s |
| `GET` | `/api/v1/apps` | Lista todas as aplicações gerenciadas |
| `POST` | `/api/v1/apps` | Faz deploy de uma aplicação (com auto-link opcional de ofertas) |
| `GET` | `/api/v1/apps/:name` | Detalhes da aplicação, pods ativos e envs injetadas |
| `DELETE`| `/api/v1/apps/:name` | Remove a aplicação e seus serviços |
| `GET` | `/api/v1/apps/:name/logs` | Exibe os logs recentes dos pods em tempo real |
| `GET` | `/api/v1/offers` | Lista catálogo de ofertas e status de instalação |
| `POST` | `/api/v1/offers/:id/install` | Instala a stack da oferta no cluster Minikube |
| `POST` | `/api/v1/apps/:name/links` | Linka uma oferta à aplicação |
| `DELETE`| `/api/v1/apps/:name/links/:offerId` | Deslinka uma oferta da aplicação |

---

## 🧪 Rodando os Testes Unitários

```bash
make test
```

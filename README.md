# NavisPaaS 🚢⚡

> **Developer-Centric Kubernetes PaaS Engine**  
> Plataforma PaaS simplificada para Kubernetes focada na experiência do desenvolvedor: faça deploy de microsserviços em namespaces dedicados e vincule ofertas de infraestrutura pré-integradas (Apache Kafka, RabbitMQ, Observabilidade Prometheus/Grafana/OTel, PostgreSQL, Redis, Kong API Gateway) com injeção automática de configurações e zero boilerplate de YAMLs.

---

## 🌟 Visão Geral e Arquitetura

O **NavisPaaS** remove a barreira de complexidade do Kubernetes para desenvolvedores. 

Em vez de escrever dezenas de arquivos de configuração (`Deployment`, `Service`, `ConfigMap`, `Secret`, `ServiceMonitor`, `KafkaTopic`), o desenvolvedor apenas informa a imagem e a porta da aplicação, marca as **ofertas** desejadas e a plataforma cuida do provisionamento, isolamento e injeção de variáveis de ambiente.

```
┌────────────────────────────────────────────────────────┐
│                   Spotify Backstage                    │
│            (Developer Portal / Scaffolder)             │
│   - Catálogo de Serviços & APIs                        │
│   - Template Self-Service com 1 clique `make backstage`│
└──────────────────────────┬─────────────────────────────┘
                           │ REST API / OpenAPI 3.0
┌──────────────────────────▼─────────────────────────────┐
│                 NavisPaaS Core Engine                  │
│                     (Backend em Go)                    │
│  - REST API (Gin + OpenAPI Spec / Swagger)             │
│  - Web Dashboard SPA Nativo (Dark Mode)                │
│  - Catalog & Service Binding Engine                    │
│  - Docker Registry Manager (imagePullSecrets)          │
└──────────────────────────┬─────────────────────────────┘
                           │ client-go / Kube API
┌──────────────────────────▼─────────────────────────────┐
│                   Cluster Minikube                     │
│  • Workload Isolado por App (ex: namespace `app-name`) │
│  • Serviços Type: LoadBalancer (Acesso via `tunnel`)   │
│  • Oferta Kafka: Apache Kafka (namespace: `kafka`)     │
│  • Oferta RabbitMQ: Message Broker (namespace: `rabbitmq`) │
│  • Oferta Observability: Prometheus + Grafana + OTel   │
│  • Oferta Dados: PostgreSQL e Redis (namespaces sep.)  │
│  • Oferta Networking: Kong API Gateway (namespace: `kong`) │
└────────────────────────────────────────────────────────┘
```

---

## ✨ Principais Funcionalidades

* 📁 **Namespace Dedicado por Aplicação**: Cada aplicação ganha seu próprio namespace isolado (ex: `football-mm`), garantindo isolamento total de Secrets, ConfigMaps, RBAC e cotas. Ao deletar o app, o namespace é limpo por completo sem deixar resíduos no cluster.
* ⚡ **Acesso Direto com Minikube Tunnel**: Os serviços são criados como `Type: LoadBalancer`. Com o comando `make tunnel`, a aplicação ganha um IP externo roteável diretamente na sua máquina host (além de manter fallback via NodePort).
* 🧩 **Catálogo de Ofertas Pré-Integradas (Addons)**:
  * **Apache Kafka**: Provisionamento de broker KRaft, tópico e injeção de `KAFKA_BOOTSTRAP_SERVERS`, `KAFKA_TOPIC`, `KAFKA_CONSUMER_GROUP`, `KAFKA_CLIENT_ID`.
  * **RabbitMQ**: Message broker com suporte AMQP, injeção de `RABBITMQ_HOST`, `RABBITMQ_PORT`, `RABBITMQ_USER`, `RABBITMQ_PASSWORD`, `RABBITMQ_URL`, Management UI na porta `30673`.
  * **Observabilidade (Prometheus + Grafana + OTel)**: Stack unificada com checkboxes para instalação individual de cada componente. Scraping automático de métricas (`prometheus.io/scrape`), injeção de endpoints do OpenTelemetry Collector e dashboards pré-configurados no Grafana (`:30080`).
  * **PostgreSQL Relational DB**: Instância PostgreSQL com injeção automática de variáveis padrão de mercado (`DATABASE_HOST`, `DATABASE_PORT`, `DATABASE_USER`, `DATABASE_PASSWORD`, `DATABASE_NAME`, `DATABASE_URL` e aliases `DB_*` e `POSTGRES_*`), compatível diretamente com TypeORM, NestJS, Prisma, Spring Boot e Django.
  * **Redis Cache**: In-memory data store com injeção de `REDIS_HOST`, `REDIS_PORT`, `REDIS_URL`.
  * **Kong API Gateway**: Gateway open-source mais popular do mercado com rate limiting, autenticação via API key, CORS e plugin ecosystem. Proxy na porta `30080`, Admin API na `30001`.
* 🐳 **Suporte a Docker Registry (Público, Privado e Minikube Local)**:
  * **Docker Hub**: Padrão nativo (`docker.io`) para imagens públicas.
  * **Registries Privados (GHCR, Harbor, ECR, GitLab)**: Gerenciamento de credenciais via UI/API com criação automática de Secret `kubernetes.io/dockerconfigjson` e injeção de `imagePullSecrets`.
  * **Docker Registry**: Configurado com `imagePullPolicy: IfNotPresent`. Para usar imagens locais sem fazer push, configure o Docker environment do seu provedor (ex: `eval $(minikube docker-env)` para Minikube).
* 🎭 **Integração Nativa com Spotify Backstage**: Inicie o portal com o comando `make backstage` com catálogo e templates do Scaffolder já pré-configurados.

---

## 🚀 Como Executar

### 1. Pré-requisitos
* Go >= 1.24
* Kubectl instalado
* Docker
* Um dos provedores Kubernetes: Minikube, k3d, Kind, Docker Desktop, Rancher Desktop ou MicroK8s

### 2. Iniciar o Cluster (escolha um)

#### Minikube (Recomendado para desenvolvimento local)
```bash
make minikube-start
make tunnel  # Em terminal separado para LoadBalancer
```

#### k3d (K3s em Docker - rápido e leve)
```bash
make k3d-start
```

#### Kind (Kubernetes em Docker)
```bash
make kind-start
```

#### Docker Desktop / Rancher Desktop
1. Habilite Kubernetes nas configurações do seu provedor
2. Execute: `kubectl config use-context docker-desktop` (ou `rancher-desktop`)

#### MicroK8s
```bash
microk8s enable dns storage
microk8s config > ~/.kube/config
```

### 3. Instalar e Iniciar o NavisPaaS

Você pode rodar o NavisPaaS compilando o código ou instalando globalmente via Go:

#### Método 1: Instalação Global (Mais Prático)
```bash
# Instala o CLI na sua máquina
go install github.com/nicolas-limadev/navis-paas/cmd/navis@latest

# Inicia o servidor e o dashboard web
navis start
```

#### Método 2: Usando o Makefile local
```bash
# Compila e roda o servidor localmente
make run
```

Acesse:
* 🌐 **Dashboard Web**: [http://localhost:8080](http://localhost:8080)
* 📚 **OpenAPI 3.0 Spec**: [http://localhost:8080/api/v1/openapi.json](http://localhost:8080/api/v1/openapi.json)

---

## 🛠️ Comandos do Makefile

| Comando | Descrição |
| :--- | :--- |
| `make build` | Compila o binário do servidor em `bin/navispaas` |
| `make build-cli` | Compila o CLI em `bin/navis` |
| `make run` | Compila e inicia o servidor do NavisPaaS na porta 8080 |
| `make minikube-start` | Inicia o Minikube local com perfil recomendado |
| `make k3d-start` | Inicia cluster k3d (K3s em Docker) |
| `make kind-start` | Inicia cluster Kind (Kubernetes em Docker) |
| `make tunnel` | Inicia o **Minikube Tunnel** para LoadBalancer |
| `make backstage` | Inicializa o **Spotify Backstage** com templates NavisPaaS |
| `make test` | Executa a suíte de testes unitários com flags verbosas |
| `make clean` | Remove os binários gerados na pasta `bin/` |

---

## 🖥️ NavisPaaS CLI

O CLI permite aos desenvolvedores gerenciar aplicações sem acessar o dashboard web.

### Instalação

```bash
# Compilar o CLI
make build-cli

# Ou instalar globalmente
go install ./cmd/navis@latest
```

### Configuração

```bash
# Criar arquivo de configuração
navis init

# Editar .navis.yml com os dados da sua aplicação
```

### Exemplo .navis.yml

```yaml
app:
  name: minha-api
  image: minha-api:v1
  port: 3000
  replicas: 2

offers:
  - name: postgresql
    enabled: true
    params:
      username: admin
      password: secret123
      databaseName: production
  - name: monitoring
    enabled: true
    components:
      prometheus: true
      grafana: true

server:
  url: http://localhost:8080
```

### Comandos

| Comando | Descrição |
| :--- | :--- |
| `navis init` | Cria arquivo `.navis.yml` de configuração |
| `navis deploy` | Deploy da aplicação baseado no `.navis.yml` |
| `navis list` | Lista aplicações deployadas |
| `navis link <app> -o <offer>` | Vincula uma oferta à aplicação |
| `navis unlink <app> <offer>` | Desvincula uma oferta |
| `navis logs <app>` | Exibe logs da aplicação |
| `navis install <offer>` | Instala componentes da oferta no cluster |
| `navis status` | Mostra status do cluster e ofertas |
| `navis delete <app>` | Remove a aplicação |

### Exemplos

```bash
# Deploy completo
navis deploy

# Instalar PostgreSQL com usuário customizado
navis install postgresql

# Linkar PostgreSQL com parâmetros
navis link minha-api -o postgresql -p username=admin,password=secret

# Ver logs
navis logs minha-api -n 50

# Ver status do cluster
navis status
```

### Variável de Ambiente

```bash
# Definir URL do servidor
export NAVIS_SERVER=http://navispaas.example.com:8080

# Ou usar flag
navis --server http://navispaas.example.com:8080 status
```

---

## 🌐 Como Acessar suas Aplicações

Para acessar os microsserviços após o deploy, o método varia conforme o provedor Kubernetes:

### Minikube
```bash
# Método 1: Minikube Tunnel (Recomendado)
make tunnel  # Em terminal separado

# Método 2: Abrir service diretamente
minikube service <nome-da-app> -n <namespace>

# Método 3: NodePort
http://192.168.49.2:<NODE_PORT>
```

### k3d / Kind / Docker Desktop
```bash
# Port-forward para acesso local
kubectl port-forward -n <namespace> svc/<app-name> 8080:<porta>
# Acesse: http://localhost:8080
```

### MicroK8s
```bash
# Port-forward
microk8s kubectl port-forward -n <namespace> svc/<app-name> 8080:<porta>
# Acesse: http://localhost:8080
```

### Todos os Provedores
O Dashboard do NavisPaaS detecta automaticamente o provedor e mostra a URL de acesso no card da aplicação.

---

## 🎭 Integração com o Spotify Backstage

O NavisPaaS fornece suporte pronto para ser o motor de execução do Backstage.

### Subir Automaticamente
Basta executar:
```bash
make backstage
```
O script `scripts/start-backstage.sh` configura o Node.js v20, instala o Yarn, cria a pasta `~/projects-github/backstage-portal` (se ainda não existir) e injeta os arquivos do NavisPaaS no `app-config.yaml`. O portal abrirá em [http://localhost:3000](http://localhost:3000).

### Adicionar em um Backstage Já Existente
Se você já possui uma instância do Backstage em execução:
1. Abra o Backstage e clique em **Create...** (ou **Catalog**).
2. Clique no canto superior direito em **Register Existing Component**.
3. Aponte para os arquivos do repositório:
   * **Componente & API**: `backstage/catalog-info.yaml`
   * **Template do Scaffolder**: `backstage/template-app.yaml`
4. Clique em **Analyze** e depois em **Import**.

---

## 📡 Referência da API REST

| Método | Endpoint | Descrição |
| :--- | :--- | :--- |
| `GET` | `/api/v1/health` | Status de saúde e conectividade com o Kubernetes |
| `GET` | `/api/v1/apps` | Lista aplicações em todos os namespaces |
| `POST` | `/api/v1/apps` | Cria aplicação no namespace dedicado e vincula ofertas |
| `GET` | `/api/v1/apps/:name` | Detalhes da aplicação, pods ativos e envs injetadas |
| `DELETE`| `/api/v1/apps/:name` | Remove a aplicação e limpa seu namespace dedicado |
| `GET` | `/api/v1/apps/:name/logs` | Streaming de logs recentes dos pods em tempo real |
| `GET` | `/api/v1/offers` | Lista ofertas disponíveis e status de instalação no cluster |
| `POST` | `/api/v1/offers/:id/install` | Instala componentes da oferta (aceita body com flags `prometheus`, `grafana`, `otel`) |
| `GET` | `/api/v1/offers/:id/status` | Retorna status dos componentes instalados (monitoring) |
| `POST` | `/api/v1/apps/:name/links` | Vincula uma oferta e injeta configurações no pod |
| `DELETE`| `/api/v1/apps/:name/links/:offerId` | Desvincula a oferta e remove variáveis do pod |
| `GET` | `/api/v1/registry` | Consulta o status das credenciais do Docker Registry |
| `POST` | `/api/v1/registry` | Salva credenciais do registry e sincroniza o secret no cluster |
| `GET` | `/api/v1/openapi.json` | Retorna a especificação OpenAPI 3.0 completa |

---

## 🧪 Testes Unitários

Para rodar a suíte completa de testes unitários:

```bash
make test
```

---

## 📦 Exemplo: Deploy Completo com Observabilidade

```bash
# 1. Instalar apenas Prometheus
curl -X POST http://localhost:8080/api/v1/offers/monitoring/install \
  -H "Content-Type: application/json" \
  -d '{"prometheus": true, "grafana": false, "otel": false}'

# 2. Adicionar Grafana depois
curl -X POST http://localhost:8080/api/v1/offers/monitoring/install \
  -H "Content-Type: application/json" \
  -d '{"prometheus": false, "grafana": true, "otel": false}'

# 3. Verificar status dos componentes
curl http://localhost:8080/api/v1/offers/monitoring/status
# Resposta: {"prometheus":true,"grafana":true,"otel":false}

# 4. Deploy da aplicação com monitoring linkado
curl -X POST http://localhost:8080/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{"name":"my-app","image":"my-image:latest","port":8080,"offers":["monitoring"]}'
```

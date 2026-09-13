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
  * **Minikube Local**: Configurado com `imagePullPolicy: IfNotPresent`. Para usar imagens locais sem fazer push, basta rodar `eval $(minikube docker-env)` antes do build.
* 🎭 **Integração Nativa com Spotify Backstage**: Inicie o portal com o comando `make backstage` com catálogo e templates do Scaffolder já pré-configurados.

---

## 🚀 Como Executar

### 1. Pré-requisitos
* Go >= 1.24
* Minikube & Kubectl instalados
* Docker

### 2. Iniciar o Cluster Minikube
```bash
make minikube-start
```

### 3. Compilar e Rodar o NavisPaaS
```bash
make run
```

Acesse:
* 🌐 **Dashboard Web**: [http://localhost:8080](http://localhost:8080)
* 📚 **OpenAPI 3.0 Spec**: [http://localhost:8080/api/v1/openapi.json](http://localhost:8080/api/v1/openapi.json)

---

## 🛠️ Comandos do Makefile

| Comando | Descrição |
| :--- | :--- |
| `make build` | Compila o binário otimizado em `bin/navispaas` |
| `make run` | Compila e inicia o servidor do NavisPaaS na porta 8080 |
| `make tunnel` | Inicia o **Minikube Tunnel** para atribuir IPs externos roteáveis às aplicações |
| `make backstage` | Inicializa o **Spotify Backstage** com os templates do NavisPaaS já plugados |
| `make test` | Executa a suíte de testes unitários com flags verbosas |
| `make minikube-start` | Inicia o Minikube local com perfil recomendado (2 CPUs, 4GB RAM) |
| `make clean` | Remove os binários gerados na pasta `bin/` |

---

## 🌐 Como Acessar suas Aplicações no Minikube

Para acessar os microsserviços após o deploy:

### Método 1: Minikube Tunnel (Recomendado)
Em um terminal separado, execute:
```bash
make tunnel
```
O Minikube cria uma rota direta na sua máquina. O Dashboard do NavisPaaS detecta o túnel e exibe o link **"Live URL"** direto no card da aplicação (ex: `http://10.96.x.x:3000`).

### Método 2: IP do Minikube + NodePort
Se não estiver usando o túnel, o NavisPaaS também aloca uma porta NodePort automaticamente:
* Descubra o IP do Minikube: `minikube ip` (ex: `192.168.49.2`)
* Acesse via: `http://<MINIKUBE_IP>:<NODE_PORT>/` (ex: `http://192.168.49.2:32215/docs/`)

### Método 3: Atalho Automático do Minikube
```bash
minikube service <nome-da-app> -n <nome-da-app>
```

> 💡 **Dica sobre a Porta do Container:** Certifique-se de preencher no formulário de deploy a porta real que sua aplicação escuta (ex: porta `3000` para NestJS/Express, `8080` para Spring Boot/Go, `80` para Nginx).

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

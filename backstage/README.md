# 🎭 Como Integrar o NavisPaaS com o Spotify Backstage

Existem duas formas simples de carregar o catálogo e os templates do **NavisPaaS** no seu Backstage:

---

## Opção 1: Se você já tem o Backstage rodando

### Método A: Via Interface Gráfica (Sem reiniciar o Backstage)
1. Acesse o seu Backstage (ex: `http://localhost:3000`).
2. No menu lateral esquerdo, clique em **Create...** (ou vá em **Catalog**).
3. No canto superior direito, clique em **Register Existing Component**.
4. No campo **URL**, cole o caminho do arquivo:
   * Para o componente e API: `/home/nicolas/projects-github/navispaas/backstage/catalog-info.yaml`
   * Para o template do Scaffolder: `/home/nicolas/projects-github/navispaas/backstage/template-app.yaml`
   *(Ou a URL raw do seu repositório no GitHub/GitLab)*.
5. Clique em **Analyze** e depois em **Import**.
6. Pronto! O template aparecerá na aba **Create...** e a API na aba **APIs**.

---

### Método B: Via `app-config.yaml` (Automático na inicialização)
Abra o arquivo `app-config.yaml` do seu projeto Backstage e adicione na seção `catalog.locations`:

```yaml
catalog:
  rules:
    - allow: [Component, System, API, Resource, Location, Template]
  locations:
    # Componente e OpenAPI do NavisPaaS
    - type: file
      target: /home/nicolas/projects-github/navispaas/backstage/catalog-info.yaml

    # Template do Scaffolder (Criação de serviços)
    - type: file
      target: /home/nicolas/projects-github/navispaas/backstage/template-app.yaml
```

Reinicie o Backstage:
```bash
yarn dev
```

---

## Opção 2: Criando um Backstage do zero (Passo a Passo)

Se você ainda não tem uma instância do Backstage na sua máquina, você já tem o **Node v20** disponível no seu sistema (`~/.nvm`):

```bash
# 1. Carregar Node.js v20 e instalar yarn
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
nvm use v20.17.0
npm install -g yarn

# 2. Criar a aplicação Backstage
cd /home/nicolas/projects-github
npx @backstage/create-app@latest --skip-install

# 3. Entrar na pasta gerada (ex: backstage) e instalar dependências
cd backstage
yarn install

# 4. Adicionar os locations do NavisPaaS no app-config.yaml (conforme Método B acima)

# 5. Iniciar o Backstage
yarn dev
```

---

## ⚡ Testando o Fluxo Completo

1. **Inicie o NavisPaaS**:
   ```bash
   cd /home/nicolas/projects-github/navispaas
   make run
   ```
2. **Abra o Backstage** em `http://localhost:3000`.
3. Vá em **Create...** -> selecione o card **"Microservice with NavisPaaS"**.
4. Preencha o nome da sua aplicação, escolha a imagem e marque:
   * `[x] Attach Observability Stack`
   * `[x] Attach Apache Kafka`
5. Clique em **Next** -> **Create**.
6. O Backstage fará a requisição para a API do NavisPaaS (`http://localhost:8080/api/v1/apps`), provisionando o Deployment, Service e os bindings no Minikube automaticamente!

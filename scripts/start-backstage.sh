#!/usr/bin/env bash
set -e

echo "🎭 Starting Spotify Backstage with NavisPaaS Catalog Integration..."

# Load NVM & Node if available
export NVM_DIR="$HOME/.nvm"
if [ -s "$NVM_DIR/nvm.sh" ]; then
  # shellcheck source=/dev/null
  \. "$NVM_DIR/nvm.sh"
  nvm use v20.17.0 2>/dev/null || nvm use default 2>/dev/null || true
fi

BACKSTAGE_DIR="$HOME/projects-github/backstage-portal"
NAVIS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Check if Backstage portal already exists
if [ ! -d "$BACKSTAGE_DIR" ]; then
  echo "📦 Backstage portal directory not found at $BACKSTAGE_DIR."
  echo "🚀 Initializing Spotify Backstage app..."
  
  if ! command -v yarn &> /dev/null; then
    echo "⚙️ Installing yarn..."
    npm install -g yarn
  fi

  cd "$HOME/projects-github"
  echo "Creating Backstage application (this may take a couple of minutes)..."
  npx @backstage/create-app@latest --skip-install backstage-portal
  
  cd "$BACKSTAGE_DIR"
  echo "📥 Installing Backstage dependencies..."
  yarn install
  
  echo "🔗 Linking NavisPaaS catalog and templates into app-config.yaml..."
  cat <<EOT >> "$BACKSTAGE_DIR/app-config.yaml"

# NavisPaaS Integration
catalog:
  locations:
    - type: file
      target: $NAVIS_DIR/backstage/catalog-info.yaml
    - type: file
      target: $NAVIS_DIR/backstage/template-app.yaml
EOT

fi

echo "🚀 Starting Spotify Backstage on http://localhost:3000..."
cd "$BACKSTAGE_DIR"
yarn dev

#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------
# IE Demo — Single EC2 setup script
# Run this on a fresh Ubuntu 24.04 instance.
#
# Prerequisites:
#   1. SSH into the instance
#   2. Clone the repo:  git clone <repo-url> ~/birch-sky
#   3. Create deploy/.env from deploy/env.example with real values
#   4. Run:  cd ~/birch-sky/deploy && bash setup.sh YOUR_DOMAIN
# ---------------------------------------------------------------

DOMAIN="${1:-}"
if [ -z "$DOMAIN" ]; then
  echo "Usage: bash setup.sh <your-domain>"
  echo "  e.g. bash setup.sh demo.infoexchange.io"
  exit 1
fi

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEPLOY_DIR="$REPO_ROOT/deploy"

# --- Preflight checks ---
if [ ! -f "$DEPLOY_DIR/.env" ]; then
  echo "ERROR: $DEPLOY_DIR/.env not found."
  echo "Copy env.example to .env and fill in real values first."
  exit 1
fi

source "$DEPLOY_DIR/.env"
if [ "$POSTGRES_PASSWORD" = "CHANGEME" ] || [ "$MINIO_ROOT_PASSWORD" = "CHANGEME" ]; then
  echo "ERROR: You haven't changed the default passwords in .env"
  echo "Generate passwords with: openssl rand -base64 24"
  exit 1
fi

if [ -z "${ANTHROPIC_API_KEY:-}" ]; then
  echo "WARNING: ANTHROPIC_API_KEY is empty. Agent harness will not work."
  read -rp "Continue anyway? [y/N] " yn
  [ "$yn" = "y" ] || exit 1
fi

echo "==> Installing Docker..."
if ! command -v docker &>/dev/null; then
  curl -fsSL https://get.docker.com | sh
  sudo usermod -aG docker "$USER"
  echo "NOTE: You may need to log out and back in for docker group to take effect."
  echo "      If 'docker compose' fails below, run: newgrp docker"
fi

echo "==> Installing Nginx + Certbot..."
sudo apt-get update -qq
sudo apt-get install -y -qq nginx certbot python3-certbot-nginx

echo "==> Building frontend..."
if ! command -v node &>/dev/null; then
  echo "    Installing Node.js 20..."
  curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
  sudo apt-get install -y -qq nodejs
fi
cd "$REPO_ROOT"
npm ci --silent
npm run build
sudo mkdir -p /var/www/ie
sudo cp -r dist/* /var/www/ie/

echo "==> Configuring Nginx..."
sudo cp "$DEPLOY_DIR/nginx-ie.conf" /etc/nginx/sites-available/ie
sudo sed -i "s/server_name _;/server_name $DOMAIN;/" /etc/nginx/sites-available/ie
sudo ln -sf /etc/nginx/sites-available/ie /etc/nginx/sites-enabled/ie
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t
sudo systemctl reload nginx

echo "==> Starting services with Docker Compose..."
cd "$DEPLOY_DIR"
docker compose \
  -f "$REPO_ROOT/src/market-platform/docker-compose.yml" \
  -f "$DEPLOY_DIR/docker-compose.prod.yml" \
  --env-file "$DEPLOY_DIR/.env" \
  up -d --build

echo "==> Waiting for services to become healthy..."
for i in $(seq 1 30); do
  if curl -sf http://127.0.0.1:8080/health >/dev/null 2>&1; then
    echo "    Market platform is healthy."
    break
  fi
  [ "$i" -eq 30 ] && echo "WARNING: market-platform did not become healthy in 60s"
  sleep 2
done

for i in $(seq 1 30); do
  if curl -sf http://127.0.0.1:8000/health >/dev/null 2>&1; then
    echo "    Agent harness is healthy."
    break
  fi
  [ "$i" -eq 30 ] && echo "WARNING: agent-harness did not become healthy in 60s"
  sleep 2
done

echo ""
echo "==> Services are up. Requesting TLS certificate..."
echo "    Make sure your DNS A record for $DOMAIN points to this server's IP."
read -rp "    Ready? [Y/n] " yn
[ "${yn:-y}" = "n" ] && { echo "Skipping certbot. Run manually: sudo certbot --nginx -d $DOMAIN"; exit 0; }

sudo certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos --register-unsafely-without-email || {
  echo ""
  echo "Certbot failed. This usually means DNS isn't pointing here yet."
  echo "Once DNS propagates, run: sudo certbot --nginx -d $DOMAIN"
}

echo ""
echo "============================================"
echo "  Deployment complete!"
echo "  https://$DOMAIN"
echo "  https://$DOMAIN/api/v1/sellers"
echo "  https://$DOMAIN/health"
echo "============================================"

# MVP AWS Deployment: Single EC2 with Docker Compose

## Architecture

```
Internet
   │
   ▼
┌──────────────────────────────────────────────────┐
│  EC2 (t3.xlarge, 4 vCPU / 16GB)                 │
│  Ubuntu 24.04 LTS                                │
│                                                  │
│  ┌─────────────────────────────────────────────┐ │
│  │ Nginx (host-installed)                      │ │
│  │  :443 ─► TLS termination (Let's Encrypt)    │ │
│  │  :80  ─► redirect to :443                   │ │
│  │                                             │ │
│  │  /              → serve /var/www/ie (static) │ │
│  │  /api/v1/*      → proxy to :8080            │ │
│  │  /agent/*       → proxy to :8000 /api/*     │ │
│  │  /health        → proxy to :8080            │ │
│  └─────────────────────────────────────────────┘ │
│                                                  │
│  ┌─── docker compose ────────────────────────┐   │
│  │ market-platform  :8080 (API) :8081 (MCP)  │   │
│  │ agent-harness    :8000                    │   │
│  │ postgres         :5432                    │   │
│  │ opensearch       :9200                    │   │
│  │ minio            :9000 / :9001            │   │
│  └───────────────────────────────────────────┘   │
│                                                  │
│  Docker volumes: pgdata, osdata, miniodata       │
└──────────────────────────────────────────────────┘
```

Everything on one box. Nginx on the host terminates TLS and reverse-proxies
to containerized services. The frontend is a static Vite build served directly
by Nginx.

## Why this is the right call for a demo

- **30-minute deploy**: one EC2, one `docker compose up`, one certbot invocation.
- **Your docker-compose.yml already works**: we reuse it almost verbatim.
- **Cheap**: a single t3.xlarge is ~$0.17/hr (~$120/mo). Kill it when not demoing.
- **Good enough security**: HTTPS, locked-down security group, real passwords, no public DB ports.
- **Easy to tear down**: one instance, one EBS volume, done.

Managed services (RDS, OpenSearch Service, ECS) are correct for production
but each one adds 10-20 min of setup time and ongoing cost. Not worth it for
a demo box.

## Prerequisites

Before you start the 30-minute clock:

1. **AWS account** with IAM permissions to create EC2, security groups, elastic IPs.
2. **A domain name** (or subdomain) you control. Point an A record to the Elastic IP
   you'll allocate. If you don't have one, you can use the raw IP with a self-signed
   cert (less polished but functional).
3. **API keys on hand**:
   - `ANTHROPIC_API_KEY` (required for agent harness)
   - `STRIPE_SECRET_KEY` (optional — stubs out if missing)
4. **SSH key pair** registered in AWS (or create one during setup).
5. **Repo cloned** or a deploy key so the EC2 can pull the code.

## Step-by-step deployment (30 minutes)

### Phase 1: AWS resources (5 min)

#### 1a. Create a Security Group (`ie-demo-sg`)

| Rule      | Port(s)   | Source        | Purpose                    |
|-----------|-----------|---------------|----------------------------|
| Inbound   | 22        | Your IP only  | SSH access                 |
| Inbound   | 80        | 0.0.0.0/0     | HTTP → redirect to HTTPS   |
| Inbound   | 443       | 0.0.0.0/0     | HTTPS (public)             |
| Outbound  | All       | 0.0.0.0/0     | Default                    |

**Nothing else is exposed.** Postgres, OpenSearch, MinIO, and the raw API ports
(8080, 8000, 9200, etc.) are only reachable from localhost inside the instance.

```bash
# Create security group
SG_ID=$(aws ec2 create-security-group \
  --group-name ie-demo-sg \
  --description "IE demo - HTTPS + SSH only" \
  --query 'GroupId' --output text)

# SSH from your IP only
MY_IP=$(curl -s https://checkip.amazonaws.com)
aws ec2 authorize-security-group-ingress \
  --group-id $SG_ID --protocol tcp --port 22 --cidr ${MY_IP}/32

# HTTP + HTTPS from anywhere
aws ec2 authorize-security-group-ingress \
  --group-id $SG_ID --protocol tcp --port 80 --cidr 0.0.0.0/0
aws ec2 authorize-security-group-ingress \
  --group-id $SG_ID --protocol tcp --port 443 --cidr 0.0.0.0/0
```

#### 1b. Launch EC2

```bash
INSTANCE_ID=$(aws ec2 run-instances \
  --image-id ami-0a0e5d9c7acc336f1 \  # Ubuntu 24.04 LTS us-east-1 (check for your region)
  --instance-type t3.xlarge \
  --key-name YOUR_KEY_NAME \
  --security-group-ids $SG_ID \
  --block-device-mappings '[{"DeviceName":"/dev/sda1","Ebs":{"VolumeSize":50,"VolumeType":"gp3"}}]' \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=ie-demo}]' \
  --query 'Instances[0].InstanceId' --output text)

# Allocate and associate Elastic IP
EIP_ALLOC=$(aws ec2 allocate-address --query 'AllocationId' --output text)
aws ec2 associate-address --instance-id $INSTANCE_ID --allocation-id $EIP_ALLOC
EIP=$(aws ec2 describe-addresses --allocation-ids $EIP_ALLOC \
  --query 'Addresses[0].PublicIp' --output text)

echo "Instance: $INSTANCE_ID  IP: $EIP"
```

**Now point your DNS A record at `$EIP`.** DNS propagation can be fast (1-2 min)
if your TTL is low.

### Phase 2: Instance setup (10 min)

SSH in and run:

```bash
ssh -i your-key.pem ubuntu@$EIP
```

#### 2a. Install Docker + Nginx

```bash
# Docker (official convenience script)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker ubuntu
newgrp docker

# Nginx + Certbot
sudo apt-get update && sudo apt-get install -y nginx certbot python3-certbot-nginx
```

#### 2b. Clone repo and configure

```bash
git clone https://github.com/harrisonengel/birch-sky.git ~/birch-sky
cd ~/birch-sky/src/market-platform
```

Create the production `.env` file:

```bash
cat > .env << 'ENVEOF'
# --- Secrets (change these!) ---
POSTGRES_PASSWORD=<generate-a-real-password>
MINIO_ROOT_USER=ieadmin
MINIO_ROOT_PASSWORD=<generate-a-real-password>
ANTHROPIC_API_KEY=<your-key>
STRIPE_SECRET_KEY=<your-key-or-leave-blank>

# --- Derived (match the passwords above) ---
DATABASE_URL=postgres://ieuser:${POSTGRES_PASSWORD}@postgres:5432/iemarket?sslmode=disable
OPENSEARCH_URL=http://opensearch:9200
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=${MINIO_ROOT_USER}
MINIO_SECRET_KEY=${MINIO_ROOT_PASSWORD}
MINIO_BUCKET=market-data
MINIO_USE_SSL=false
HTTP_PORT=8080
MCP_PORT=8081
OPENSEARCH_INDEX=listings
MODEL_NAME=claude-sonnet-4-5
ENVEOF
```

> **Generate passwords with**: `openssl rand -base64 24`

### Phase 3: Production docker-compose overlay (5 min)

Create `docker-compose.prod.yml` alongside the existing file. This overrides
dev defaults with real credentials and adds restart policies:

```yaml
# docker-compose.prod.yml
services:
  postgres:
    environment:
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    restart: unless-stopped

  opensearch:
    environment:
      - "OPENSEARCH_JAVA_OPTS=-Xms1g -Xmx1g"   # more RAM on xlarge
    restart: unless-stopped

  minio:
    environment:
      MINIO_ROOT_USER: ${MINIO_ROOT_USER}
      MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD}
    restart: unless-stopped

  market-platform:
    environment:
      DATABASE_URL: ${DATABASE_URL}
      MINIO_ACCESS_KEY: ${MINIO_ACCESS_KEY}
      MINIO_SECRET_KEY: ${MINIO_SECRET_KEY}
    restart: unless-stopped

  agent-harness:
    environment:
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY}
    restart: unless-stopped
```

Start everything:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

Verify:

```bash
# Wait for health checks
docker compose ps   # all should show "healthy"
curl -s http://localhost:8080/health   # {"status":"ok"}
curl -s http://localhost:8000/health   # {"status":"ok"}
```

### Phase 4: Frontend build + Nginx (5 min)

#### 4a. Build the frontend

```bash
cd ~/birch-sky
npm ci && npm run build
sudo mkdir -p /var/www/ie
sudo cp -r dist/* /var/www/ie/
```

#### 4b. Configure Nginx

```bash
sudo tee /etc/nginx/sites-available/ie << 'NGEOF'
server {
    listen 80;
    server_name YOUR_DOMAIN;   # e.g. demo.infoexchange.io

    root /var/www/ie;
    index index.html;

    # Frontend - SPA fallback
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Market platform API
    location /api/v1/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Health endpoint
    location /health {
        proxy_pass http://127.0.0.1:8080;
    }

    # Agent harness (rewrite /agent/* → /api/*)
    location /agent/ {
        rewrite ^/agent/(.*)$ /api/$1 break;
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Agent requests may take a while
        proxy_read_timeout 120s;
    }
}
NGEOF

sudo ln -sf /etc/nginx/sites-available/ie /etc/nginx/sites-enabled/ie
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t && sudo systemctl reload nginx
```

### Phase 5: TLS with Let's Encrypt (5 min)

```bash
sudo certbot --nginx -d YOUR_DOMAIN --non-interactive --agree-tos -m your@email.com
```

Certbot auto-configures the Nginx server block for HTTPS and sets up
auto-renewal via a systemd timer.

Verify: `https://YOUR_DOMAIN` should load the frontend and
`https://YOUR_DOMAIN/api/v1/health` should return `{"status":"ok"}`.

## Post-deploy smoke test

```bash
# From your laptop:
curl -s https://YOUR_DOMAIN/health
curl -s https://YOUR_DOMAIN/api/v1/sellers
curl -s -X POST https://YOUR_DOMAIN/api/v1/sellers \
  -H 'Content-Type: application/json' \
  -d '{"name":"Demo Seller","description":"Test seller"}'
```

Then open the site in a browser and run through the buyer flow.

## Security checklist (demo-grade)

- [x] HTTPS via Let's Encrypt (auto-renewing)
- [x] Security group: only 22/80/443 open; SSH restricted to your IP
- [x] Postgres/OpenSearch/MinIO not exposed to internet
- [x] Real passwords (not `iepass` / `minioadmin`)
- [x] API keys passed via `.env`, not committed to git
- [x] `restart: unless-stopped` so services survive reboots
- [ ] **Not done (not needed for demo)**: WAF, rate limiting, Cognito enforcement,
      DB encryption at rest, VPC private subnets, backup strategy

## Cost

| Resource         | Cost          |
|------------------|---------------|
| t3.xlarge        | ~$0.17/hr     |
| 50GB gp3 EBS    | ~$4/mo        |
| Elastic IP (in use) | Free      |
| **Running 24/7** | **~$126/mo** |
| **Running 8hr/day** | **~$42/mo** |

Stop the instance when not demoing to save money:
```bash
aws ec2 stop-instances --instance-ids $INSTANCE_ID
aws ec2 start-instances --instance-ids $INSTANCE_ID  # Elastic IP stays bound
```

## What to do when this outgrows a single box

When you need real scale or multi-tenant isolation, the migration path is:

1. **PostgreSQL → RDS** (managed backups, failover)
2. **OpenSearch → Amazon OpenSearch Service** (managed cluster)
3. **MinIO → S3** (already S3-compatible, swap endpoint)
4. **market-platform + agent-harness → ECS Fargate** (auto-scaling containers)
5. **Nginx → ALB + ACM** (managed load balancer + TLS)
6. **Add Cognito enforcement** at the ALB level

Each of these is an independent migration. You don't have to do them all at once.

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

### Phase 2: Instance setup — the automated way (15 min)

All of phases 2-5 are automated by `deploy/setup.sh`. The config files
it uses are checked into the repo:

| File                            | Purpose                                         |
|---------------------------------|-------------------------------------------------|
| `deploy/setup.sh`              | One-shot setup script (Docker, Nginx, TLS, etc) |
| `deploy/docker-compose.prod.yml` | Compose overlay: real creds, localhost-only ports, restart policies |
| `deploy/nginx-ie.conf`         | Nginx reverse proxy + SPA config                |
| `deploy/env.example`           | Template for the `.env` secrets file             |

SSH in:

```bash
ssh -i your-key.pem ubuntu@$EIP
```

Clone the repo and create your `.env`:

```bash
git clone https://github.com/harrisonengel/birch-sky.git ~/birch-sky
cd ~/birch-sky/deploy
cp env.example .env
nano .env   # Fill in real passwords and API keys
            # Generate passwords with: openssl rand -base64 24
```

Run the setup script:

```bash
bash setup.sh YOUR_DOMAIN
```

The script:
1. Validates your `.env` (rejects default passwords)
2. Installs Docker, Nginx, Certbot, Node.js
3. Builds the frontend and copies it to `/var/www/ie`
4. Configures Nginx with the checked-in config
5. Starts all services via `docker compose` with the prod overlay
6. Waits for health checks
7. Requests a TLS certificate via Let's Encrypt

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

# AWS Account Setup Checklist

> **Account:** IE (Information Exchange)
> **Primary Region:** us-west-2 (Seattle)
> **Last Updated:** 2026-04-14

---

## P0 — Do This Now

### Root Account Lockdown
- [ ] Enable MFA on root account (hardware key preferred, TOTP via 1Password acceptable)
- [ ] Remove any root access keys if they exist (`Security Credentials` → `Access Keys`)
- [ ] Store root credentials (email + password + MFA recovery) in 1Password vault
- [ ] Set a strong unique password on root (generate via 1Password)
- [ ] Stop using root for all day-to-day work
- [ ] Add an alternate contact email under `Account` → `Alternate Contacts`

### IAM User / Identity Center Setup
- [ ] Decide: IAM user (simpler) vs Identity Center/SSO (scales better)
- [ ] **If IAM user path:**
  - [ ] Create `Admin` IAM group, attach `AdministratorAccess` managed policy
  - [ ] Create personal IAM user (`harrison`) with console access
  - [ ] Add user to `Admin` group
  - [ ] Enable MFA on IAM user
  - [ ] Generate access key pair, store in 1Password
  - [ ] Log out of root, log in as `harrison`, verify everything works
- [ ] **If Identity Center path:**
  - [ ] Enable IAM Identity Center in us-west-2
  - [ ] Create permission set with `AdministratorAccess`
  - [ ] Create user, assign to account with admin permission set
  - [ ] Configure `aws sso login` in CLI
  - [ ] Verify console + CLI access works

### IAM Policy Skeleton
- [ ] Create `Deploy` IAM role (for CI/CD — GitHub Actions, etc.)
  - [ ] Trust policy: GitHub OIDC provider (avoid long-lived keys in CI)
  - [ ] Permissions: scope to services the deploy pipeline touches (build as you go)
- [ ] Create service execution roles as needed (ECS task role, Lambda role, etc.)
  - [ ] Each role gets only the permissions its workload needs
  - [ ] Use resource-level ARN scoping where possible (specific buckets, tables, etc.)

### Billing & Cost Controls
- [ ] Enable Cost Explorer (`Billing` → `Cost Explorer`)
- [ ] Create AWS Budget: alert at $100/month
- [ ] Create AWS Budget: alert at $500/month (something is wrong)
- [ ] Add billing alert email (personal + IE email if applicable)
- [ ] Review and understand Free Tier usage dashboard
- [ ] Enable Cost Anomaly Detection (catches runaway resources automatically)

---

## P1 — Do This Week

### Security & Monitoring
- [ ] Verify CloudTrail is enabled in us-west-2 (on by default for management events)
- [ ] Create a dedicated S3 bucket for CloudTrail logs with versioning enabled
- [ ] Configure CloudTrail to log to that bucket
- [ ] Enable GuardDuty in us-west-2 (one click, cheap, catches compromised creds)
- [ ] Enable S3 Block Public Access at the **account level** (`S3` → `Block Public Access settings for this account`)
- [ ] Enable EBS default encryption in us-west-2

### Region Lockdown
- [ ] Decide which regions you'll use (probably just us-west-2 to start)
- [ ] Create IAM policy or SCP to deny actions in all other regions
  - Exception: global services (IAM, CloudFront, Route53, S3 global)
- [ ] Apply to your Admin group or as an SCP if using Organizations

### Account Configuration
- [ ] Set account alias for friendlier login URL (`IAM` → `Account Alias`, e.g. `ie-prod`)
- [ ] Enable IAM Access Analyzer in us-west-2 (finds overly permissive policies and public resources)
- [ ] Review and clean up default VPC security groups (remove `0.0.0.0/0` inbound rules)

---

## P2 — Do Before Production

### Networking
- [ ] Create a custom VPC for production workloads
  - [ ] CIDR block: e.g. `10.0.0.0/16`
  - [ ] Public subnets (2+ AZs) for load balancers
  - [ ] Private subnets (2+ AZs) for application + database tiers
  - [ ] NAT Gateway for outbound internet from private subnets
- [ ] Configure security groups with least-privilege inbound rules
- [ ] Consider VPC Flow Logs to S3 for network audit trail

### Secrets & Configuration
- [ ] Set up AWS Secrets Manager or SSM Parameter Store for application secrets
- [ ] Never store secrets in environment variables, code, or .env files in git
- [ ] Create a rotation policy for any long-lived credentials
- [ ] Add `aws-vault` or `granted` to your local CLI workflow for credential management

### DNS & Domain
- [ ] Set up Route53 hosted zone for IE domain(s)
- [ ] Enable DNSSEC if appropriate
- [ ] Point domain registrar nameservers to Route53

### CI/CD Security
- [ ] Set up GitHub OIDC identity provider in IAM (avoids long-lived keys)
- [ ] Create scoped deploy role with trust policy for your GitHub org/repo
- [ ] Test deploy pipeline with temporary credentials
- [ ] Verify no secrets are logged in CI output

### Data
- [ ] Enable S3 versioning on all production buckets
- [ ] Enable S3 lifecycle rules (transition old versions to Glacier, expire after N days)
- [ ] Enable RDS automated backups if using RDS
- [ ] Enable DynamoDB point-in-time recovery if using DynamoDB
- [ ] Document backup and disaster recovery plan

---

## P3 — Do Before Bringing On People

### Multi-Account Strategy
- [ ] Create AWS Organization
- [ ] Separate accounts for: production, staging, and shared services / billing
- [ ] Apply SCPs at the organization level for guardrails
- [ ] Centralize CloudTrail and GuardDuty findings to management account

### Team IAM
- [ ] Move to Identity Center / SSO if not already
- [ ] Create permission sets: Admin, Developer, ReadOnly
- [ ] Create break-glass procedure for emergency root access
- [ ] Document IAM onboarding / offboarding checklist for new team members
- [ ] Enable credential reports and review them monthly

### Compliance & Audit
- [ ] Enable AWS Config for resource compliance tracking
- [ ] Set up Config rules for common checks (e.g. S3 buckets encrypted, EBS encrypted)
- [ ] Enable Security Hub and review findings
- [ ] Schedule quarterly IAM access review

---

## Credential Hygiene Reminders

- **Never** commit AWS keys to git (add to `.gitignore`: `.env`, `*.pem`, `credentials`)
- **Never** paste keys in Slack, email, or shared docs
- **Prefer** temporary credentials (SSO, OIDC, STS AssumeRole) over long-lived access keys
- **Rotate** any long-lived access keys at least every 90 days
- **Use** `aws-vault`, `granted`, or `aws sso login` for local development
- **Audit** active access keys monthly via IAM Credential Report

---

## Quick Reference

| Task | How often | Where |
|------|-----------|-------|
| Review billing | Weekly | Billing → Cost Explorer |
| Check GuardDuty findings | Weekly | GuardDuty console |
| Review IAM Access Analyzer | Monthly | IAM → Access Analyzer |
| Rotate access keys | Every 90 days | IAM → Users → Security Credentials |
| Review CloudTrail | As needed | CloudTrail → Event History |
| Full security review | Quarterly | Security Hub |

---

## Notes

_Use this space to track decisions, deferred items, or context._

-

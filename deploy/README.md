# deploy/

How the three services run in the hosted staging and production environments:
one arm64 EC2 VM runs both environments as separate Docker Compose projects,
next to managed Postgres (RDS), MongoDB (Atlas) and S3 + CloudFront for the
SPA and media. The decision behind this layout is ADR-039 in motifpath-specs.
The cloud resources themselves are Terraform in motifpath-infra. This
directory covers what runs on the VM and how a release reaches it.

| File | What it is |
|---|---|
| `compose.yaml` | One environment: Redpanda + the three services, limits, CloudWatch logging. Run as project `staging` or `production`. |
| `env/<environment>.env` | That environment's non-secret settings: hostnames, loopback ports, sizing. |
| `edge/` | The Caddy reverse proxy, its own Compose project `edge`. Terminates TLS for both environments. |
| `deploy.sh` | Runs on the VM. Renders secrets, pulls, recreates, reloads the edge, health-checks. |

## Release flow

```
feat/…  →  dev  →  stage  →  main
                    │         │
                 staging   production
```

1. **Merging into `stage`** runs `Build images`: the three images are built
   for `linux/arm64` on a native arm runner and pushed to ECR as
   `motifpath/<service>:<commit-sha>`. The workflow then calls `Deploy` for
   **staging** with that SHA.
2. **Check staging** at `https://staging.motifpath.com`.
3. **Merge `stage` into `main`**, then run `Deploy` by hand with
   `environment: production` and the **SHA staging ran**, not the merge
   commit. The workflow refuses a SHA that is not on `main`.
4. **Hotfix:** merging a `hotfix/…` PR into `main` builds that merge commit.
   Deploy it to production by hand (staging first if it changes the schema).
5. **Rollback:** run `Deploy` with the previous SHA. The VM records the last
   two in `/etc/motifpath/<environment>.deployed`.

Containers are recreated in place, so each service is down for a few
seconds per deploy. core-domain applies pending migrations before it starts
listening, so every migration runs against staging's data before production's.

## What the infrastructure has to provide

None of this exists yet. Both workflows fail early with a pointer to this
file until it does.

### On the VM

- Amazon Linux 2023 (arm64) with Docker, the Compose v2 plugin, the AWS CLI,
  `python3` and `curl`, and the SSM agent running.
- **IMDSv2 hop limit of 2**, so the AWS SDK inside a container can reach the
  instance role.
- An instance role that can:
  - `ssm:GetParametersByPath` on `/motifpath/staging/*`, `/motifpath/production/*`
    and `/motifpath/edge/*`, plus `kms:Decrypt` on their key
  - `ecr:GetAuthorizationToken`, plus pull on the three `motifpath/*` repositories
  - read and write each environment's media bucket
  - `logs:CreateLogStream` and `logs:PutLogEvents` on the log groups below
- Inbound 80 and 443 only. No SSH: shell access goes through Session Manager.

### CloudWatch log groups

`/motifpath/staging`, `/motifpath/production` and `/motifpath/edge`. The
logging driver does not create them.

### ECR

Repositories `motifpath/core-domain`, `motifpath/event-ingestion` and
`motifpath/aggregation-worker`, with immutable tags.

### SSM Parameter Store

Every parameter directly under each path is written into that environment's
secrets file, with its last path segment as the variable name. Use
`SecureString` for anything secret.

| Parameter | Example |
|---|---|
| `/motifpath/<env>/DATABASE_URL` | `postgres://motifpath_<env>:…@<rds-host>:5432/motifpath_<env>?sslmode=require` |
| `/motifpath/<env>/MONGO_URI` | the Atlas connection string for that environment's cluster |
| `/motifpath/<env>/CLERK_SECRET_KEY` | `sk_live_…` (production), `sk_test_…` (staging) |
| `/motifpath/<env>/ECR_REGISTRY` | `<account-id>.dkr.ecr.us-east-1.amazonaws.com` |
| `/motifpath/<env>/MEDIA_S3_BUCKET` | that environment's media bucket name |
| `/motifpath/edge/ACME_EMAIL` | the address Let's Encrypt sends expiry notices to |

A value cannot contain a single quote or a newline; `deploy.sh` refuses it
rather than write a broken env file.

### GitHub

- An IAM OIDC provider for `token.actions.githubusercontent.com` and two roles:
  - **ECR push**, trusted for this repo's `stage` and `main` refs. Set its ARN
    as the repository variable `AWS_ECR_PUSH_ROLE_ARN`.
  - **Deploy**, trusted for the `staging` and `production` GitHub
    environments, allowed `ssm:SendCommand` with `AWS-RunShellScript` on the
    VM only, `ssm:GetCommandInvocation`, and `ecr:DescribeImages`.
- GitHub environments `staging` and `production`, each with the variables
  `AWS_DEPLOY_ROLE_ARN` and `VM_INSTANCE_ID`. Add required reviewers to
  `production` if a second pair of eyes should approve each release.
- A protected `stage` branch.

### DNS

`api.`, `events.`, `api.staging.` and `events.staging.motifpath.com`, plus
the apex and `www`, as A records to the VM's Elastic IP. Caddy can only get
certificates once those resolve.

## Running it by hand

Over Session Manager, as root, from an unpacked bundle:

```bash
bash /opt/motifpath/staging/deploy.sh staging <commit-sha>
```

To see what is running:

```bash
docker compose ls
docker compose -p staging ps
cat /etc/motifpath/staging.deployed
```

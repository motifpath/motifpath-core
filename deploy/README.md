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

The deploy workflow never sends these files or any command to the VM. It
sends the environment's SSM deploy document (`motifpath-<env>-deploy`, defined
in motifpath-infra) and a commit SHA. The VM checks that the SHA is on the
environment's release branches (`main` for production; `stage` or `main` for
staging), downloads this directory from that commit on GitHub, and runs
`deploy.sh`. Only code merged to those branches ever runs on the host that
both environments share.

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
3. **Merge `stage` into `main`** with a merge commit.
4. **Release:** tag that merge commit on `main` with the next version and push
   the tag. Anything without a tag is an intermediate build: a release
   candidate on `stage`, or a staging-only deploy.

   ```bash
   git fetch origin main && git tag -a v0.2.0 origin/main -m "v0.2.0" && git push origin v0.2.0
   ```

   The tag starts `Deploy` for **production**, which waits for the
   `production` environment's approval if it has reviewers. The merge commit
   has no images, so it deploys the newest commit in the tag that was built
   in all three repositories: the `stage` commit staging ran. The images get
   `production-<sha>` and the release tag in ECR (`production-` images are
   kept out of the expiry of older release candidates). Once production runs
   it, a GitHub Release is published with notes since the previous tag. A tag
   that is not on `main` is refused, here and on the VM.
5. **Hotfix:** merging a `hotfix/…` PR into `main` builds that merge commit.
   Once `Build images` has finished, tag it with the next patch version
   (staging first if it changes the schema).
6. **Rollback or redeploy:** run `Deploy` with `environment: production` and
   `release` set to an earlier tag. The VM records the last two SHAs in
   `/etc/motifpath/<environment>.deployed`. In an emergency `release` also
   takes a full commit SHA; that deploy is not a release and publishes
   nothing.

   ```bash
   gh workflow run deploy.yml -R motifpath/motifpath-core --ref main -f environment=production -f release=v0.1.0
   ```

Versions are this repository's own (motifpath-web numbers its releases
separately). Old release images expire with the rest of the `production-`
images, so only the most recent few releases can be rolled back to.

Containers are recreated in place, so each service is down for a few
seconds per deploy. core-domain applies pending migrations before it starts
listening, so every migration runs against staging's data before production's.

## What the infrastructure has to provide

None of this exists yet. Both workflows fail early with a pointer to this
file until it does. The Terraform that creates it, and the one-time setup
steps, are in motifpath-infra.

### On the VM

- Amazon Linux 2023 (arm64) with Docker, the Compose v2 plugin, the AWS CLI,
  `python3` and `curl`, and the SSM agent running.
- **IMDSv2 hop limit of 1**, so containers cannot reach the instance role;
  only the host uses it. Services that need AWS get their own credentials
  (core-domain's per-environment media key).
- An instance role that can:
  - `ssm:GetParametersByPath` on `/motifpath/staging/*`, `/motifpath/production/*`
    and `/motifpath/edge/*`
  - `ecr:GetAuthorizationToken`, plus pull on the three `motifpath/*` repositories
  - `logs:CreateLogStream` and `logs:PutLogEvents` on the log groups below
- An SSM document `motifpath-<env>-deploy` per environment, as described above.
- Inbound 80 and 443 only. No SSH: shell access goes through Session Manager.

### CloudWatch log groups

`/motifpath/staging`, `/motifpath/production` and `/motifpath/edge`. The
logging driver does not create them.

### ECR

Repositories `motifpath/core-domain`, `motifpath/event-ingestion` and
`motifpath/aggregation-worker`, with immutable tags. Images tagged
`production-*` expire only among themselves.

### SSM Parameter Store

Every parameter directly under each path is written into that environment's
secrets file, with its last path segment as the variable name. Use
`SecureString` for anything secret.

| Parameter | Example |
|---|---|
| `/motifpath/<env>/DATABASE_URL` | `postgres://motifpath_<env>:…@<rds-host>:5432/motifpath_<env>?sslmode=require` |
| `/motifpath/<env>/MONGO_URI` | the Atlas connection string for that environment's cluster |
| `/motifpath/<env>/CLERK_SECRET_KEY` | `sk_live_…` (production), `sk_test_…` (staging) |
| `/motifpath/<env>/ECR_REGISTRY` | `<account-id>.dkr.ecr.us-east-2.amazonaws.com` |
| `/motifpath/<env>/MEDIA_S3_BUCKET` | that environment's media bucket name |
| `/motifpath/<env>/MEDIA_AWS_ACCESS_KEY_ID` | access key of the IAM user limited to that bucket |
| `/motifpath/<env>/MEDIA_AWS_SECRET_ACCESS_KEY` | its secret |
| `/motifpath/<env>/EMAIL_UNSUBSCRIBE_SECRET` | random; signs the unsubscribe links in emails, so changing it breaks every link already sent |
| `/motifpath/<env>/SMTP_HOST` | `email-smtp.us-east-2.amazonaws.com` |
| `/motifpath/<env>/SMTP_USERNAME` | access key ID of the IAM user that may send only from that environment's mail domain |
| `/motifpath/<env>/SMTP_PASSWORD` | the SES SMTP password derived from its secret |
| `/motifpath/<env>/MAIL_FROM_ADDRESS` | `noreply@mail.motifpath.com` (production), `noreply@mail.staging.motifpath.com` (staging) |
| `/motifpath/<env>/CONCIERGE_EMAIL` | the address the weekly student summary goes to (`hello@motifpath.com`), from the environment's `concierge_email` in Terraform. Without it, none is sent |
| `/motifpath/edge/ACME_EMAIL` | the address Let's Encrypt sends expiry notices to |

A value cannot contain a single quote or a newline; `deploy.sh` refuses it
rather than write a broken env file.

### GitHub

The AWS project does not allow GitHub's OIDC identity provider, so CI signs
in with IAM-user access keys kept as GitHub **environment** secrets
(`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`). Each key belongs to a user
scoped to one job, and each environment is limited under **Deployment
branches** so other branches cannot read its secrets:

| Environment | Deployment branches | Key can | Also |
|---|---|---|---|
| `images` | `stage`, `main` | push to the three `motifpath/*` repositories | — |
| `staging` | `stage`, `main` | send only staging's deploy document; `ecr:DescribeImages` | variable `VM_INSTANCE_ID` |
| `production` | `main`, and **tags** `v*` | send only production's deploy document; `ecr:DescribeImages`; add the `production-` and release tags (`ecr:BatchGetImage`, `ecr:PutImage`) | variable `VM_INSTANCE_ID`; required reviewers recommended |

A release runs on its tag, not on a branch, so `production`'s rule is
**Selected branches and tags** with `main` and the tag pattern `v*`. Protect
the tags with a ruleset on `refs/tags/v*` that blocks updating and deleting
them and lets only repository admins create them: a moved tag would make a
release name point at different code, and creating one deploys production.

A build refuses to push over an existing tag, so an image published early
with a leaked push key fails the build instead of being deployed.

- A protected `stage` branch.

### DNS

`api.`, `events.`, `api.staging.` and `events.staging.motifpath.com`, plus
the apex and `www`, as A records to the VM's Elastic IP. Caddy can only get
certificates once those resolve.

## Running it by hand

Over Session Manager, as root, re-running the last unpacked deploy:

```bash
bash /opt/motifpath/staging/deploy.sh staging <commit-sha>
```

To see what is running:

```bash
docker compose ls
docker compose -p staging ps
cat /etc/motifpath/staging.deployed
```

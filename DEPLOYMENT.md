# Ripple backend deployment

Ripple can run on Cloud Run with Supabase Postgres. Keep the Supabase database URL and JWT signing secret in Google Secret Manager. The GitHub deployment workflow uses Workload Identity Federation, so it does not need a long-lived Google service-account key.

## 1. Select the Google Cloud project

Sign in and explicitly select the Google Cloud project that should own Ripple’s resources.

```sh
gcloud auth login
gcloud projects list
export PROJECT_ID="ripple-be-20261010-farras"
export REGION="asia-southeast2"
gcloud config set project "$PROJECT_ID"
gcloud config set run/region "$REGION"
```

The `ripple-be-20261010-farras` project is linked to billing. Cloud Run is available in Jakarta (`asia-southeast2`). The deployment APIs are enabled:

```sh
gcloud services enable \
  run.googleapis.com \
  artifactregistry.googleapis.com \
  secretmanager.googleapis.com \
  iam.googleapis.com \
  iamcredentials.googleapis.com \
  sts.googleapis.com \
  --project="$PROJECT_ID"
```

Enabling APIs requires the **Service Usage Admin** role (`roles/serviceusage.serviceUsageAdmin`). Creating secrets later requires **Secret Manager Admin** (`roles/secretmanager.admin`). If either command returns `PERMISSION_DENIED`, ask the project owner to enable the API or grant the needed role before continuing.

## 2. Prepare Supabase

In Supabase, open **Connect** and copy a PostgreSQL connection string. For Cloud Run, use the **Session pooler** connection when using the shared pooler; it is reachable over IPv4 and supports persistent backend connections. Add `sslmode=require` to the URL and URL-encode special characters in the password. Keep the pooler host and username exactly as Supabase provides them.

Create the required secrets without putting their values in shell history:

```sh
gcloud secrets create ripple-db-addr --replication-policy=automatic
read -rs "SUPABASE_DB_ADDR?Paste the Supabase session-pooler URL: "
printf '\n'
printf '%s' "$SUPABASE_DB_ADDR" | gcloud secrets versions add ripple-db-addr --data-file=-
unset SUPABASE_DB_ADDR

openssl rand -base64 48 | gcloud secrets create ripple-jwt-secret --replication-policy=automatic --data-file=-
```

If a secret already exists, omit its `gcloud secrets create` command and add a new version with `gcloud secrets versions add`.

Apply the migrations to Supabase before the first deploy. This runs every migration in this repository against the database URL you provide:

```sh
read -rs "SUPABASE_DB_ADDR?Paste the Supabase session-pooler URL: "
printf '\n'
migrate -path=cmd/migrate/migrations -database="$SUPABASE_DB_ADDR" up
unset SUPABASE_DB_ADDR
```

Back up any existing Supabase data before applying migrations to a database that is already in use.

## 3. Create Cloud Run identities and Artifact Registry

```sh
gcloud artifacts repositories create ripple \
  --project="$PROJECT_ID" \
  --location="$REGION" \
  --repository-format=docker

export RUNTIME_SERVICE_ACCOUNT="ripple-api-runtime@${PROJECT_ID}.iam.gserviceaccount.com"
export DEPLOY_SERVICE_ACCOUNT="ripple-github-deployer@${PROJECT_ID}.iam.gserviceaccount.com"
```

Allow the runtime identity to read the two required secrets:

```sh
for secret in ripple-db-addr ripple-jwt-secret; do
  gcloud secrets add-iam-policy-binding "$secret" \
    --project="$PROJECT_ID" \
    --member="serviceAccount:${RUNTIME_SERVICE_ACCOUNT}" \
    --role=roles/secretmanager.secretAccessor
done
```

Optional providers use the same environment variable names as `.envrc`. Add those keys to Secret Manager only when configured, then attach them to Cloud Run with `gcloud run services update --update-secrets`:

| Secret Manager secret | Runtime environment variable |
| --- | --- |
| `ripple-hf-api-key` | `HF_API_KEY` |
| `ripple-openai-api-key` | `OPENAI_API_KEY` |
| `ripple-r2-access-key-id` | `R2_ACCESS_KEY_ID` |
| `ripple-r2-secret-access-key` | `R2_SECRET_ACCESS_KEY` |

Grant the runtime identity access to each optional secret you create, then attach the ones you use:

```sh
for secret in ripple-hf-api-key ripple-openai-api-key ripple-r2-access-key-id ripple-r2-secret-access-key; do
  gcloud secrets add-iam-policy-binding "$secret" \
    --project="$PROJECT_ID" \
    --member="serviceAccount:${RUNTIME_SERVICE_ACCOUNT}" \
    --role=roles/secretmanager.secretAccessor
done

gcloud run services update ripple-api \
  --project="$PROJECT_ID" \
  --region="$REGION" \
  --update-secrets="HF_API_KEY=ripple-hf-api-key:latest,OPENAI_API_KEY=ripple-openai-api-key:latest,R2_ACCESS_KEY_ID=ripple-r2-access-key-id:latest,R2_SECRET_ACCESS_KEY=ripple-r2-secret-access-key:latest"
```

Only run this block after you have created all four optional secrets. If you only configure some providers, omit the other secret references from `--update-secrets`.

Set the non-secret `R2_ACCOUNT_ID` and `R2_BUCKET` values as Cloud Run environment variables if R2 attachments are enabled. Set `OPENAI_MODEL` in GitHub repository variables for ChatGPT brief summaries and planning. Leave unset providers disabled.

## 4. Build and deploy with gcloud

Grant your own signed-in user access to Artifact Registry and Cloud Run if needed. Then build and push the image:

```sh
gcloud auth configure-docker "${REGION}-docker.pkg.dev"
export IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/ripple/ripple-api:initial"
docker build --pull -t "$IMAGE" .
docker push "$IMAGE"
```

Deploy the API. Cloud Run invocation is public so a browser-based local frontend can call it; Ripple still enforces its own JWT authentication on protected API routes.

```sh
gcloud run deploy ripple-api \
  --project="$PROJECT_ID" \
  --region="$REGION" \
  --platform=managed \
  --image="$IMAGE" \
  --port=8080 \
  --allow-unauthenticated \
  --service-account="$RUNTIME_SERVICE_ACCOUNT" \
  --min=0 \
  --max=2 \
  --concurrency=40 \
  --cpu=1 \
  --memory=512Mi \
  --timeout=300 \
  --set-env-vars="ENV=production,ADDR=:8080,FRONTEND_URL=http://localhost:3000,OPENAI_MODEL=${OPENAI_MODEL:-},R2_ACCOUNT_ID=${R2_ACCOUNT_ID:-},R2_BUCKET=${R2_BUCKET:-},AI_IMAGE_ESTIMATE_CENTS=1,AI_VIDEO_ESTIMATE_CENTS=200,AI_IMAGE_REFINEMENT_ESTIMATE_CENTS=1,AI_VIDEO_REFINEMENT_ESTIMATE_CENTS=2000" \
  --set-secrets="DB_ADDR=ripple-db-addr:latest,JWT_SECRET=ripple-jwt-secret:latest"
```

Get the service URL and check health:

```sh
export API_URL="$(gcloud run services describe ripple-api --project="$PROJECT_ID" --region="$REGION" --format='value(status.url)')"
curl "$API_URL/api/v1/health"
```

For a local frontend, copy its `.env.example` to `.env.local`, set `NEXT_PUBLIC_API_URL="$API_URL/api/v1"`, then restart the Next.js dev server. The Go API already permits `http://localhost:3000` and `http://127.0.0.1:3000` through CORS.

## 5. Configure GitHub Actions CI/CD with Workload Identity Federation

Create the GitHub deployment identity and a restricted OIDC provider for this repository:

```sh
gcloud iam workload-identity-pools create github \
  --project="$PROJECT_ID" \
  --location=global \
  --display-name="GitHub Actions"

gcloud iam workload-identity-pools providers create-oidc ripple-be \
  --project="$PROJECT_ID" \
  --location=global \
  --workload-identity-pool=github \
  --display-name="Ripple backend repository" \
  --issuer-uri="https://token.actions.githubusercontent.com" \
  --attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository,attribute.ref=assertion.ref" \
  --attribute-condition="assertion.repository == 'farrasnazhif/ripple-be' && assertion.ref == 'refs/heads/main'"

export PROJECT_NUMBER="$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')"
export WIF_POOL="projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/github"

gcloud iam service-accounts add-iam-policy-binding "$DEPLOY_SERVICE_ACCOUNT" \
  --project="$PROJECT_ID" \
  --role=roles/iam.workloadIdentityUser \
  --member="principalSet://iam.googleapis.com/${WIF_POOL}/attribute.repository/farrasnazhif/ripple-be"

gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:${DEPLOY_SERVICE_ACCOUNT}" \
  --role=roles/run.admin

gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:${DEPLOY_SERVICE_ACCOUNT}" \
  --role=roles/serviceusage.serviceUsageConsumer

gcloud artifacts repositories add-iam-policy-binding ripple \
  --project="$PROJECT_ID" \
  --location="$REGION" \
  --member="serviceAccount:${DEPLOY_SERVICE_ACCOUNT}" \
  --role=roles/artifactregistry.writer

gcloud iam service-accounts add-iam-policy-binding "$RUNTIME_SERVICE_ACCOUNT" \
  --project="$PROJECT_ID" \
  --member="serviceAccount:${DEPLOY_SERVICE_ACCOUNT}" \
  --role=roles/iam.serviceAccountUser

gcloud secrets add-iam-policy-binding ripple-db-addr \
  --project="$PROJECT_ID" \
  --member="serviceAccount:${DEPLOY_SERVICE_ACCOUNT}" \
  --role=roles/secretmanager.secretAccessor
```

Read the provider name for the GitHub repository settings:

```sh
gcloud iam workload-identity-pools providers describe ripple-be \
  --project="$PROJECT_ID" \
  --location=global \
  --workload-identity-pool=github \
  --format='value(name)'
```

The workflow has defaults for this Google Cloud project and repository, so it can run without GitHub variables. To override them, open **Settings → Secrets and variables → Actions → Variables** and add:

| Variable | Value |
| --- | --- |
| `GCP_PROJECT_ID` | `ripple-be-20261010-farras` |
| `GCP_REGION` | `asia-southeast2` |
| `GCP_AR_REPOSITORY` | `ripple` |
| `GCP_DEPLOY_SERVICE_ACCOUNT` | `ripple-github-deployer@ripple-be-20261010-farras.iam.gserviceaccount.com` |
| `GCP_RUNTIME_SERVICE_ACCOUNT` | `ripple-api-runtime@ripple-be-20261010-farras.iam.gserviceaccount.com` |
| `GCP_WORKLOAD_IDENTITY_PROVIDER` | Full provider name printed above |
| `FRONTEND_URL` | Optional deployed frontend origin; defaults to `http://localhost:3000` |
| `OPENAI_MODEL` | Your configured OpenAI model name, or leave unset |
| `R2_ACCOUNT_ID` | Your Cloudflare account ID, if R2 is configured |
| `R2_BUCKET` | Your R2 bucket name, if R2 is configured |

The deployment maps the app’s environment this way: `ADDR` and `ENV` are fixed for Cloud Run; `DB_ADDR` and `JWT_SECRET` come from Secret Manager; `FRONTEND_URL`, `OPENAI_MODEL`, `R2_ACCOUNT_ID`, and `R2_BUCKET` are non-secret GitHub variables; `OPENAI_API_KEY`, `HF_API_KEY`, and R2 access keys are optional Secret Manager values; generation estimates are set in the workflow. `FRONTEND_URL` defaults to `http://localhost:3000`, and the backend CORS middleware also allows `http://127.0.0.1:3000`.

The `SUPABASE_DB_ADDR` GitHub secret is optional. If absent, the workflow reads `ripple-db-addr` from Secret Manager using the deploy identity. Push to `main` to build and publish the container, apply database migrations, and deploy the new Cloud Run revision. The workflow is in `.github/workflows/deploy.yml`.

The workflow deploys with the current image estimates (one cent per image request); video rates remain zero and therefore disabled until you choose and configure those estimates.

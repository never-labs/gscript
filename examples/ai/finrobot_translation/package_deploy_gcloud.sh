#!/usr/bin/env sh
set -eu

SERVICE="${1:-leia-finrobot-translation}"
REGION="${2:-us-central1}"
IMAGE="${3:-gcr.io/${GOOGLE_CLOUD_PROJECT:-PROJECT_ID}/${SERVICE}}"

docker build \
  -f examples/ai/finrobot_translation/package_deploy_Dockerfile \
  -t "${IMAGE}" \
  .

docker push "${IMAGE}"

gcloud run deploy "${SERVICE}" \
  --image "${IMAGE}" \
  --region "${REGION}" \
  --platform managed \
  --allow-unauthenticated \
  --set-env-vars LEIA_FINROBOT_DATA_DIR=/app/examples/ai/finrobot_translation

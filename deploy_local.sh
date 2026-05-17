#!/usr/bin/env bash
set -euo pipefail
# ====== CONFIG ======
MSG_UPDATE="${MSG_UPDATE:-update}"
BRANCH="${BRANCH:-main}"
PLATFORM="${PLATFORM:-linux/amd64}"


# .env optional (export all)
if [[ -f ".env" ]]; then
  set -a
  source .env
  set +a
fi

: "${PORTAINER_URL:?missing}"
: "${PORTAINER_API_KEY:?missing}"
: "${ENDPOINT_ID:?missing}"
: "${APP_NAME:?missing}"
: "${EXTERNAL_PORT:?missing}"
: "${CONFIG_YAML_B64:?missing}"

# ====== Git: commit + push ======
echo "==> git pull"
git pull origin "$BRANCH"

echo "==> git add/commit"
git add .
if git diff --cached --quiet; then
  echo "==> No staged changes. Skip commit."
else
  git commit -m "$MSG_UPDATE"
fi

echo "==> fetch tags"
git fetch --tags

LAST_TAG="$(git tag | sort -V | tail -n 1)"
if [[ -z "${LAST_TAG}" ]]; then
  LAST_TAG="v0.0.0"
fi
echo "==> latest tag: ${LAST_TAG}"

IFS='.' read -r MAJOR MINOR PATCH <<< "${LAST_TAG//v/}"
PATCH=$((PATCH + 1))
NEW_TAG="v${MAJOR}.${MINOR}.${PATCH}"
echo "==> new tag: ${NEW_TAG}"

echo "==> create tag"
git tag "${NEW_TAG}"

echo "==> push branch + tag"
git push origin "$BRANCH"
git push origin "${NEW_TAG}"

# ====== Docker buildx: ensure builder ======
echo "==> ensure docker buildx builder"
if ! docker buildx version >/dev/null 2>&1; then
  echo "Docker buildx not available. Please install/update Docker Desktop."
  exit 1
fi

BUILDER_NAME="amd64builder"
if ! docker buildx inspect "$BUILDER_NAME" >/dev/null 2>&1; then
  docker buildx create --name "$BUILDER_NAME" --use >/dev/null
else
  docker buildx use "$BUILDER_NAME" >/dev/null
fi

# (Optional) boot builder
docker buildx inspect --bootstrap >/dev/null

# ====== Build amd64 image locally (important: --load) ======
IMAGE_TAG="${NEW_TAG}"
IMAGE_NAME="${APP_NAME}:${IMAGE_TAG}"
OUTPUT_TAR="${APP_NAME}-${IMAGE_TAG}.tar"

echo "==> Build docker image: ${IMAGE_NAME} (${PLATFORM})"
docker buildx build \
  --platform "${PLATFORM}" \
  -t "${IMAGE_NAME}" \
  --load \
  .

echo "==> Save docker image to tar: ${OUTPUT_TAR}"
docker save -o "${OUTPUT_TAR}" "${IMAGE_NAME}"

echo "==> Upload image tar to Portainer (endpoint ${ENDPOINT_ID})"
curl -fS -X POST \
  "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/v1.44/images/load" \
  -H "X-API-Key: ${PORTAINER_API_KEY}" \
  -H "Content-Type: application/x-tar" \
  --data-binary "@${OUTPUT_TAR}"

echo "==> Find existing container named ${APP_NAME}"
CONTAINERS_JSON="$(curl -fsS \
  "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/json?all=true" \
  -H "X-API-Key: ${PORTAINER_API_KEY}")"

CONTAINER_ID="$(echo "${CONTAINERS_JSON}" | jq -r --arg name "/${APP_NAME}" '
  .[] | select(.Names != null) | select(.Names | index($name)) | .Id
' | head -n 1)"

if [[ -n "${CONTAINER_ID}" && "${CONTAINER_ID}" != "null" ]]; then
  echo "==> Container exists (${CONTAINER_ID}). Stopping + removing..."
  curl -fsS -X POST \
    "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/${CONTAINER_ID}/stop" \
    -H "X-API-Key: ${PORTAINER_API_KEY}" || true

  curl -fsS -X DELETE \
    "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/${CONTAINER_ID}?force=true" \
    -H "X-API-Key: ${PORTAINER_API_KEY}"
else
  echo "==> No existing container found. Creating new."
fi

echo "==> Create container ${APP_NAME} from image ${IMAGE_NAME}"

ENV_JSON="$(jq -cn \
  --arg tz "Asia/Bangkok" \
  --arg cfg "${CONFIG_YAML_B64}" \
  --arg env "${ENV:-}" '
  [
    "TZ=\($tz)",
    "API_CONFIG_PATH=/app/config",
    "API_CONFIG_NAME=config",
    "CONFIG_YAML_B64=\($cfg)",
    "ENV=\($env)"
  ]
')"

CREATE_BODY="$(jq -cn \
  --arg image "${IMAGE_NAME}" \
  --argjson env "${ENV_JSON}" \
  --arg hostPort "${EXTERNAL_PORT}" '
  {
    Image: $image,
    Env: $env,
    ExposedPorts: {"8080/tcp": {}},
    HostConfig: {
      PortBindings: {"8080/tcp": [{HostPort: $hostPort}]},
      RestartPolicy: {Name: "always"}
    },
    Cmd: [
      "sh","-lc",
      "mkdir -p /app/config && echo \"$CONFIG_YAML_B64\" | base64 -d > /app/config/config.yaml && exec uvicorn app.main:app --host 0.0.0.0 --port 8080"
    ]
  }
')"

CREATE_RES="$(curl -fsS -X POST \
  "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/create?name=${APP_NAME}" \
  -H "X-API-Key: ${PORTAINER_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "${CREATE_BODY}")"

NEW_ID="$(echo "${CREATE_RES}" | jq -r '.Id')"
echo "==> Created container id: ${NEW_ID}"

echo "==> Start container"
curl -fsS -X POST \
  "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/${NEW_ID}/start" \
  -H "X-API-Key: ${PORTAINER_API_KEY}"

echo "==> Done. ${APP_NAME} running on :${EXTERNAL_PORT} -> 8080"
rm -f "${OUTPUT_TAR}"
echo "==> Cleanup docker image ${IMAGE_NAME}"
docker rmi "${IMAGE_NAME}" || true
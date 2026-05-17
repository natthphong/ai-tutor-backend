#!/usr/bin/env bash
set -euo pipefail

: "${PORTAINER_URL:?missing}"
: "${PORTAINER_API_KEY:?missing}"
: "${ENDPOINT_ID:?missing}"
: "${APP_NAME:?missing}"
: "${EXTERNAL_PORT:?missing}"
: "${CONFIG_YAML_B64:?missing}"

IMAGE_TAG="${IMAGE_TAG:-${GITHUB_REF_NAME:-latest}}"
IMAGE_NAME="${APP_NAME}:${IMAGE_TAG}"
OUTPUT_TAR="${APP_NAME}-${IMAGE_TAG}.tar"

echo "==> Build Go binary"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o goapp

echo "==> Build docker image: ${IMAGE_NAME}"
docker build -t "${IMAGE_NAME}" .

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
  '
  [
    "TZ=\($tz)",
    "API_CONFIG_PATH=/app/config",
    "API_CONFIG_NAME=config",
    "CONFIG_YAML_B64=\($cfg)"
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
      "mkdir -p /app/config && echo \"$CONFIG_YAML_B64\" | base64 -d > /app/config/config.yaml && exec /app/goapp"
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
rm -f "${OUTPUT_TAR}" goapp

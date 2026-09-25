#!/usr/bin/env bash
# Build the backup image and verify it uploads a zip to MinIO via the mock Outline server.
set -euo pipefail

cleanup() {
  docker rm -f minio mock-outline-server >/dev/null 2>&1 || true
  docker volume rm outline-backup-temp >/dev/null 2>&1 || true
  docker network rm outline-backup-test >/dev/null 2>&1 || true
}

cleanup
trap cleanup EXIT

docker network create outline-backup-test

# quay.io/minio/minio:latest-cicd defaults to the bare `minio` command, which
# exits without opening a port, so pass `server /data`.
docker run -d \
  --name minio \
  --network outline-backup-test \
  -p 9000:9000 \
  -p 9001:9001 \
  -e MINIO_ROOT_USER=minio \
  -e MINIO_ROOT_PASSWORD=minio123 \
  -e MINIO_ACCESS_KEY=minio \
  -e MINIO_SECRET_KEY=minio123 \
  quay.io/minio/minio:latest-cicd \
  server /data --console-address ":9001"

if ! timeout 60 bash -c 'until curl -sf http://127.0.0.1:9000/minio/health/live >/dev/null; do sleep 2; done'; then
  echo "MinIO did not become ready" >&2
  docker logs minio || true
  exit 1
fi
echo "MinIO is ready"

# dl.min.io returns HTTP 410. The client is published on GitHub releases.
mc_release="RELEASE.2025-08-13T08-35-41Z"
case "$(uname -m)" in
  x86_64) mc_arch="amd64" ;;
  aarch64 | arm64) mc_arch="arm64" ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac
mc_url="https://github.com/minio/mc/releases/download/${mc_release}/mc.linux-${mc_arch}.${mc_release}"
mkdir -p "${HOME}/minio-binaries"
curl -fL "$mc_url" -o "${HOME}/minio-binaries/mc"
chmod +x "${HOME}/minio-binaries/mc"
mc="${HOME}/minio-binaries/mc"
"$mc" --version

"$mc" alias set myminio http://127.0.0.1:9000 minio minio123
"$mc" mb myminio/outline-test
if ! "$mc" policy set public myminio/outline-test; then
  "$mc" anonymous set public myminio/outline-test
fi

docker build -t mock-outline-server --load mock-outline-server
docker build -t outlinewikibackup --load .

docker run -d \
  --name mock-outline-server \
  --network outline-backup-test \
  -p 3000:3000 \
  mock-outline-server

docker volume create outline-backup-temp
docker run --rm -v outline-backup-temp:/tmp alpine:latest chown -R 65534:65534 /tmp

docker run --rm \
  --network outline-backup-test \
  -v outline-backup-temp:/tmp \
  -e API_BASE_URL='http://mock-outline-server:3000' \
  -e AUTH_TOKEN='test-token' \
  -e AWS_ACCESS_KEY_ID='minio' \
  -e AWS_SECRET_ACCESS_KEY='minio123' \
  -e MINIO_ENDPOINT='http://minio:9000' \
  -e S3_BUCKET_NAME='outline-test' \
  -e UPLOAD_TO_S3='true' \
  -e KEEP_BACKUPS='3' \
  outlinewikibackup

if "$mc" ls myminio/outline-test | grep -q ".zip"; then
  echo "Integration test passed: Found backup(s) in MinIO bucket"
else
  echo "Integration test failed: No backups found in MinIO bucket" >&2
  exit 1
fi

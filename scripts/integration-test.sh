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

docker run -d \
  --name minio \
  --network outline-backup-test \
  -p 9000:9000 \
  -p 9001:9001 \
  -e MINIO_ROOT_USER=minio \
  -e MINIO_ROOT_PASSWORD=minio123 \
  minio/minio:latest \
  server /data --console-address ":9001"

timeout 60 bash -c 'until curl -f http://127.0.0.1:9000/minio/health/live; do sleep 2; done'
echo "MinIO is ready"

arch="$(uname -m)"
case "$arch" in
  x86_64) mc_url="https://dl.min.io/client/mc/release/linux-amd64/mc" ;;
  aarch64 | arm64) mc_url="https://dl.min.io/client/mc/release/linux-arm64/mc" ;;
  *)
    echo "Unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

curl --fail --silent --show-error --location "$mc_url" \
  --create-dirs \
  --output "$HOME/minio-binaries/mc"
chmod +x "$HOME/minio-binaries/mc"
"$HOME/minio-binaries/mc" --version

"$HOME/minio-binaries/mc" alias set myminio http://127.0.0.1:9000 minio minio123
"$HOME/minio-binaries/mc" mb myminio/outline-test
"$HOME/minio-binaries/mc" policy set public myminio/outline-test

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

if "$HOME/minio-binaries/mc" ls myminio/outline-test | grep -q ".zip"; then
  echo "Integration test passed: Found backup(s) in MinIO bucket"
else
  echo "Integration test failed: No backups found in MinIO bucket" >&2
  exit 1
fi

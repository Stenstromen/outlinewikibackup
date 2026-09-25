#!/usr/bin/env bash
# Build the backup image and verify it uploads a zip to Garage via the mock Outline server.
set -euo pipefail

if [[ -n "${CONTAINER_RUNTIME:-}" ]]; then
  runtime="$CONTAINER_RUNTIME"
elif command -v podman >/dev/null 2>&1 && ! command -v docker >/dev/null 2>&1; then
  runtime="podman"
elif command -v docker >/dev/null 2>&1; then
  runtime="docker"
elif command -v podman >/dev/null 2>&1; then
  runtime="podman"
else
  echo "docker or podman is required" >&2
  exit 1
fi

# Garage v2.3 access keys are GK plus 32 hex characters. The secret is 64 hex characters.
access_key="GK0123456789abcdef0123456789abcdef"
secret_key="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
bucket="outline-test"
config_dir=""

cleanup() {
  "$runtime" rm -f garage mock-outline-server >/dev/null 2>&1 || true
  "$runtime" volume rm outline-backup-temp >/dev/null 2>&1 || true
  "$runtime" network rm outline-backup-test >/dev/null 2>&1 || true
  if [[ -n "$config_dir" ]]; then
    rm -rf "$config_dir"
  fi
}
cleanup
trap cleanup EXIT
config_dir="$(mktemp -d)"

container_build() {
  local tag="$1"
  local context="$2"
  if [[ "$runtime" == "podman" ]]; then
    "$runtime" build -t "$tag" "$context"
  else
    "$runtime" build -t "$tag" --load "$context"
  fi
}

cat > "${config_dir}/garage.toml" <<'EOF'
metadata_dir = "/tmp/meta"
data_dir = "/tmp/data"
db_engine = "sqlite"
replication_factor = 1

rpc_bind_addr = "[::]:3901"
rpc_public_addr = "127.0.0.1:3901"
rpc_secret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

[s3_api]
s3_region = "garage"
api_bind_addr = "[::]:3900"
root_domain = ".s3.garage.localhost"

[s3_web]
bind_addr = "[::]:3902"
root_domain = ".web.garage.localhost"
index = "index.html"

[admin]
api_bind_addr = "[::]:3903"
admin_token = "outlinewikibackup-test-admin"
metrics_token = "outlinewikibackup-test-metrics"
EOF

"$runtime" network create outline-backup-test

# --single-node assigns a layout. --default-bucket creates the key and bucket.
"$runtime" run -d \
  --name garage \
  --network outline-backup-test \
  -p 3900:3900 \
  -v "${config_dir}/garage.toml:/etc/garage.toml:ro" \
  -e GARAGE_DEFAULT_ACCESS_KEY="$access_key" \
  -e GARAGE_DEFAULT_SECRET_KEY="$secret_key" \
  -e GARAGE_DEFAULT_BUCKET="$bucket" \
  dxflrs/garage:v2.3.0 \
  /garage server --single-node --default-bucket

wait_until() {
  local seconds="$1"
  shift
  local deadline=$((SECONDS + seconds))
  until "$@"; do
    if ((SECONDS >= deadline)); then
      return 1
    fi
    sleep 2
  done
}

if ! wait_until 60 curl -s -o /dev/null --max-time 2 http://127.0.0.1:3900/; then
  echo "Garage did not become ready" >&2
  "$runtime" logs garage || true
  exit 1
fi
bucket_ready() {
  "$runtime" exec garage /garage bucket list | grep -q outline-test
}
if ! wait_until 30 bucket_ready; then
  echo "Garage bucket was not created" >&2
  "$runtime" logs garage || true
  exit 1
fi
echo "Garage is ready"

mc_release="RELEASE.2025-08-13T08-35-41Z"
case "$(uname -s)" in
  Linux) mc_os="linux" ;;
  Darwin) mc_os="darwin" ;;
  *)
    echo "Unsupported operating system: $(uname -s)" >&2
    exit 1
    ;;
esac
case "$(uname -m)" in
  x86_64) mc_arch="amd64" ;;
  aarch64 | arm64) mc_arch="arm64" ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac
mc_url="https://github.com/minio/mc/releases/download/${mc_release}/mc.${mc_os}-${mc_arch}.${mc_release}"
mkdir -p "${HOME}/minio-binaries"
mc="${HOME}/minio-binaries/mc.${mc_os}-${mc_arch}"
curl -fL "$mc_url" -o "$mc"
chmod +x "$mc"

MC_REGION=garage "$mc" alias set mygarage http://127.0.0.1:3900 "$access_key" "$secret_key" --api S3v4

container_build mock-outline-server mock-outline-server
container_build outlinewikibackup .

"$runtime" run -d \
  --name mock-outline-server \
  --network outline-backup-test \
  -p 3000:3000 \
  mock-outline-server

"$runtime" volume create outline-backup-temp
"$runtime" run --rm -v outline-backup-temp:/tmp alpine:latest chown -R 65534:65534 /tmp

"$runtime" run --rm \
  --network outline-backup-test \
  -v outline-backup-temp:/tmp \
  -e API_BASE_URL='http://mock-outline-server:3000' \
  -e AUTH_TOKEN='test-token' \
  -e AWS_ACCESS_KEY_ID="$access_key" \
  -e AWS_SECRET_ACCESS_KEY="$secret_key" \
  -e GARAGE_ENDPOINT='http://garage:3900' \
  -e S3_BUCKET_NAME="$bucket" \
  -e UPLOAD_TO_S3='true' \
  -e KEEP_BACKUPS='3' \
  outlinewikibackup

if MC_REGION=garage "$mc" ls "mygarage/${bucket}" | grep -q ".zip"; then
  echo "Integration test passed: Found backup(s) in Garage bucket"
else
  echo "Integration test failed: No backups found in Garage bucket" >&2
  "$runtime" logs garage || true
  exit 1
fi

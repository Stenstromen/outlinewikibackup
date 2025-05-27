IMAGE_NAME = outlinewikibackup
IMAGE_TAG = latest
MINIO_ACCESS_KEY = minio
MINIO_SECRET_KEY = minio123
MINIO_BUCKET = outline-test
NETWORK_NAME = outline-backup-test
MINIO_CONTAINER = minio-test
TEMP_VOLUME = outline-backup-temp

.PHONY: build test clean test-deps network minio-deploy

test-deps:
	@which podman >/dev/null 2>&1 || (echo "❌ podman is required but not installed. Aborting." && exit 1)

build:
	@echo "ℹ️ Building application image..."
	podman build -t localhost/$(IMAGE_NAME):$(IMAGE_TAG) .

network:
	@echo "ℹ️ Creating podman network $(NETWORK_NAME)..."
	podman network create $(NETWORK_NAME) || true

minio-deploy: test-deps network
	@echo "ℹ️ Cleaning up any existing MinIO container..."
	podman stop $(MINIO_CONTAINER) 2>/dev/null || true
	podman rm $(MINIO_CONTAINER) 2>/dev/null || true

	@echo "ℹ️ Starting MinIO container without persistent storage..."
	podman run -dt \
		--name $(MINIO_CONTAINER) \
		--network $(NETWORK_NAME) \
		-p 9000:9000 \
		-e "MINIO_ACCESS_KEY=$(MINIO_ACCESS_KEY)" \
		-e "MINIO_SECRET_KEY=$(MINIO_SECRET_KEY)" \
		docker.io/minio/minio server /data

	@echo "ℹ️ Waiting for MinIO to initialize..."
	sleep 10

	@echo "ℹ️ Verifying MinIO server status..."
	podman logs $(MINIO_CONTAINER)

	@echo "ℹ️ Creating bucket using MinIO Client..."
	podman run --rm --entrypoint /bin/sh \
		--network $(NETWORK_NAME) \
		docker.io/minio/mc -c " \
		mc alias set myminio http://$(MINIO_CONTAINER):9000 $(MINIO_ACCESS_KEY) $(MINIO_SECRET_KEY) && \
		mc mb myminio/$(MINIO_BUCKET) && \
		mc policy set public myminio/$(MINIO_BUCKET) \
		"
	
	@echo "ℹ️ Creating temporary volume for backup data..."
	podman volume create $(TEMP_VOLUME) || true

test: build minio-deploy
	@echo "ℹ️ Running backup test..."
	podman run --rm \
		--network $(NETWORK_NAME) \
		-v $(TEMP_VOLUME):/tmp \
		-e API_BASE_URL='$(API_BASE_URL)' \
		-e AUTH_TOKEN='$(AUTH_TOKEN)' \
		-e AWS_ACCESS_KEY_ID='$(MINIO_ACCESS_KEY)' \
		-e AWS_SECRET_ACCESS_KEY='$(MINIO_SECRET_KEY)' \
		-e MINIO_ENDPOINT='http://$(MINIO_CONTAINER):9000' \
		-e S3_BUCKET_NAME='$(MINIO_BUCKET)' \
		-e UPLOAD_TO_S3='true' \
		-e KEEP_BACKUPS='3' \
		localhost/$(IMAGE_NAME):$(IMAGE_TAG)

	@echo "ℹ️ Verifying backup files in MinIO..."
	podman run --rm --entrypoint /bin/sh \
		--network $(NETWORK_NAME) \
		docker.io/minio/mc -c " \
		mc alias set myminio http://$(MINIO_CONTAINER):9000 $(MINIO_ACCESS_KEY) $(MINIO_SECRET_KEY) && \
		mc ls myminio/$(MINIO_BUCKET) \
		"
	
	@if podman run --rm --entrypoint /bin/sh \
		--network $(NETWORK_NAME) \
		docker.io/minio/mc -c " \
		mc alias set myminio http://$(MINIO_CONTAINER):9000 $(MINIO_ACCESS_KEY) $(MINIO_SECRET_KEY) && \
		mc ls myminio/$(MINIO_BUCKET) \
		" | grep -q ".zip"; then \
		echo "✅ Integration test passed: Found backup(s) in MinIO bucket"; \
	else \
		echo "❌ Integration test failed: No backups found in MinIO bucket"; \
		exit 1; \
	fi

clean:
	@echo "ℹ️ Cleaning up containers, volumes, and network..."
	podman stop $(MINIO_CONTAINER) || true
	podman rm $(MINIO_CONTAINER) || true
	podman volume rm $(TEMP_VOLUME) || true
	podman network rm $(NETWORK_NAME) || true 
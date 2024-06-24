# OutlineWiki Backup

![OutlineWikiLogo](outlinewikibackup_logo.webp)

## Description

This is a Go binary to backup an OutlineWiki instance. It uses the OutlineWiki API to export the data and saves it locally. Optionally, it can upload the backup to an S3/MinIO bucket.

This project was inspired by the lack of a built-in backup feature in OutlineWiki, and the need to have a backup of the data in case of data loss. Also, as a response to the lack of any practical backup examples in the [OutlineWiki documentation](https://docs.getoutline.com/s/hosting/doc/backups-KZtPOADCHG).

## Usage

### Create API Key in OutlineWiki

1. Go to the OutlineWiki instance.
2. Click on the user icon in the lower-left corner.
3. Click on "Preferences".
4. Click on "API".
5. Click on "+ New API Key".

### Run backup to MinIO bucket using Podman

```bash
podman run --rm \
-e API_BASE_URL='https://myoutlinewiki.example.com' \
-e AUTH_TOKEN='ol_api_abcd1234' \
-e AWS_ACCESS_KEY_ID='AKIA5XQW2PQEEK5FKYFS' \
-e AWS_SECRET_ACCESS_KEY='s3cr3t' \
-e MINIO_ENDPOINT='https://minio.example.com' \
-e S3_BUCKET_NAME='outline' \
-e UPLOAD_TO_S3='true' \
ghcr.io/stenstromen/outlinewikibackup:latest
```

## Environment Variables

- `SAVE_DIR`: The directory to save the file locally, defaults to `/tmp/outlinewikibackups` if not set.
- `UPLOAD_TO_S3`: If set to `"true"`, the file will be uploaded to S3/MinIO.
- `S3_BUCKET_NAME`: The S3/MinIO bucket name.
- `AWS_REGION`: The AWS region, required if not using MinIO.
- `MINIO_ENDPOINT`: The MinIO endpoint URL, required if using MinIO.
- `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`: Credentials for AWS S3 or MinIO.
- `KEEP_BACKUPS` (optional): The number of backups to keep, defaults to infinite.
- `SLEEP_DURATION` (optional): The duration to sleep (wait) before checking export status, defaults to 10 seconds.

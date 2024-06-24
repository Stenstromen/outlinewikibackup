# OutlineWiki Backup

## Environment Variables

- `SAVE_DIR`: The directory to save the file locally, defaults to `/tmp/outlinewikibackups` if not set.
- `UPLOAD_TO_S3`: If set to `"true"`, the file will be uploaded to S3/MinIO.
- `S3_BUCKET_NAME`: The S3/MinIO bucket name.
- `AWS_REGION`: The AWS region, required if not using MinIO.
- `MINIO_ENDPOINT`: The MinIO endpoint URL, required if using MinIO.
- `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`: Credentials for AWS S3 or MinIO.
- `KEEP_BACKUPS` (optional): The number of backups to keep, defaults to infinite.
- `SLEEP_DURATION` (optional): The duration to sleep (wait) before checking export status, defaults to 10 seconds.

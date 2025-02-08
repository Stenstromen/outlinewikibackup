# OutlineWiki Backup

![OutlineWikiLogo](outlinewikibackup_logo.webp)

- [OutlineWiki Backup](#outlinewiki-backup)
  - [Description](#description)
  - [Usage](#usage)
    - [Create API Key in OutlineWiki](#create-api-key-in-outlinewiki)
    - [Run backup to MinIO bucket using Podman](#run-backup-to-minio-bucket-using-podman)
    - [Restore from Backup](#restore-from-backup)
    - [Example Kubernetes Cronjob](#example-kubernetes-cronjob)
  - [Environment Variables](#environment-variables)

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
-v /tmp:/tmp:rw \
-e API_BASE_URL='https://myoutlinewiki.example.com' \
-e AUTH_TOKEN='ol_api_abcd1234' \
-e AWS_ACCESS_KEY_ID='AKIA5XQW2PQEEK5FKYFS' \
-e AWS_SECRET_ACCESS_KEY='s3cr3t' \
-e MINIO_ENDPOINT='https://minio.example.com' \
-e S3_BUCKET_NAME='outline' \
-e UPLOAD_TO_S3='true' \
ghcr.io/stenstromen/outlinewikibackup:latest
```

### Restore from Backup

1. Go to the OutlineWiki instance.
2. Click on the user icon in the lower-left corner.
3. Click on "Import".
4. Select a previously exported backup file.
5. Click on "StartImport".

### Example Kubernetes Cronjob

MinIO requirements for the Kubernetes CronJob are:

1. A running MinIO instance.
1. A bucket named outline.
1. The access key ID and secret access key for the MinIO instance.
1. The MinIO instance should be accessible at the specified endpoint.

```yaml
apiVersion: v1
data:
  auth-token: b2xfYXBpX2FiY2QxMjM0
  minio-access-key-id: QUtJQTVYUVcyUFFFRUs1RktZRlM=
  minio-secret-access-key: czNjcjN0
kind: Secret
metadata:
  name: outline-backup-secrets
  namespace: default
type: Opaque

---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: outline-backup
  namespace: default
spec:
  concurrencyPolicy: Forbid
  failedJobsHistoryLimit: 1
  jobTemplate:
    spec:
      activeDeadlineSeconds: 3600
      backoffLimit: 2
      template:
        spec:
          containers:
            - env:
                - name: KEEP_BACKUPS
                  value: "7"
                - name: API_BASE_URL
                  value: https://outline.example.com
                - name: S3_BUCKET_NAME
                  value: outline
                - name: UPLOAD_TO_S3
                  value: "true"
                - name: MINIO_ENDPOINT
                  value: https://minio.example.com
                - name: AUTH_TOKEN
                  valueFrom:
                    secretKeyRef:
                      key: auth-token
                      name: outline-backup-secrets
                - name: AWS_ACCESS_KEY_ID
                  valueFrom:
                    secretKeyRef:
                      key: minio-access-key-id
                      name: outline-backup-secrets
                - name: AWS_SECRET_ACCESS_KEY
                  valueFrom:
                    secretKeyRef:
                      key: minio-secret-access-key
                      name: outline-backup-secrets
              securityContext:
                runAsUser: 65534
                runAsGroup: 65534
                privileged: false
                runAsNonRoot: true
                readOnlyRootFilesystem: true
                allowPrivilegeEscalation: false
                procMount: Default
                capabilities:
                  drop: ["ALL"]
                seccompProfile:
                  type: RuntimeDefault
              image: ghcr.io/stenstromen/outlinewikibackup:latest
              imagePullPolicy: IfNotPresent
              name: outline-backup
              terminationMessagePath: /dev/termination-log
              terminationMessagePolicy: File
              volumeMounts:
                - name: tmp
                  mountPath: /tmp
          dnsPolicy: ClusterFirst
          restartPolicy: Never
          schedulerName: default-scheduler
          terminationGracePeriodSeconds: 30
          volumes:
            - name: tmp
              emptyDir: {}
  schedule: "@daily"
  successfulJobsHistoryLimit: 0
  suspend: false
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

FROM golang:1.22-alpine as build
WORKDIR /
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags='-w -s' -o /outlinewikibackup

FROM alpine:latest
RUN addgroup -S outlinewikibackup && adduser -S outlinewikibackup -G outlinewikibackup
WORKDIR /usr/src/app
COPY --from=build /outlinewikibackup /usr/src/app/outlinewikibackup
USER outlinewikibackup
CMD ["./outlinewikibackup"]
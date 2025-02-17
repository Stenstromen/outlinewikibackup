FROM golang:1.24-alpine as build
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags='-w -s' -installsuffix cgo -o /outlinewikibackup ./

FROM scratch
COPY --from=build /outlinewikibackup /
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 65534:65534
CMD ["/outlinewikibackup"]
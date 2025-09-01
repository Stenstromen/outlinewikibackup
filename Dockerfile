FROM golang:1.25-alpine as build
WORKDIR /app
COPY . .
# Enable experimental garbage collector for better performance
RUN CGO_ENABLED=0 GOOS=linux GOEXPERIMENT=greenteagc go build -a -ldflags='-w -s' -installsuffix cgo -o /outlinewikibackup ./

FROM scratch
COPY --from=build /outlinewikibackup /
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 65534:65534
CMD ["/outlinewikibackup"]
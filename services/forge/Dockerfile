# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.24.10-bookworm@sha256:aaa1dedbfef846dbd62fa803cecd7486b0e98a32cb6f68656eee20acac5dd53a AS supervisor
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY internal/forge ./internal/forge
COPY cmd/forge-service ./cmd/forge-service
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -buildvcs=false -o /forge-service ./cmd/forge-service
FROM eclipse-temurin@sha256:24a8854594eea72c16822953e6cb96c78d10fc3c77b7b8a60ce8e5ac440a2337
COPY --from=forge-runtime / /opt/forge/
COPY --from=supervisor /forge-service /usr/local/bin/forge-service
COPY --chmod=0555 services/forge/launch /opt/forge/launch
USER 65532:65532
ENV PORT=8081 FORGE_MAX_GAMES=1 FORGE_BUNDLE=/opt/forge FORGE_DEFAULTS_DIR=/opt/forge/defaults FORGE_SESSIONS_DIR=/tmp/manatomb-games
EXPOSE 8081
STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/forge-service"]

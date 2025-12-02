# Build stage
FROM golang:1.24 AS build


WORKDIR /app

# First copy go.mod/go.sum and download deps (for better Docker layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Now copy the rest of the source
COPY . .

# Build the binary
# Adjust path to main package if needed (cmd/server)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o manatomb ./cmd/server

# Runtime stage
FROM gcr.io/distroless/base-debian12

WORKDIR /app

# Copy binary from build stage
COPY --from=build /app/manatomb /app/manatomb

# Copy templates (and any other assets your app loads from disk)
COPY --from=build /app/internal/web/templates /app/internal/web/templates

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/manatomb"]
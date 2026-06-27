# oci-start Go rewrite — Dockerfile (pure-Go static binary, no cgo).
# Go 1.25: oci-go-sdk/v65 v65.118.1 requires go >= 1.24 (Phase 3 bump).
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Default build uses the stub SPA embed; for a production image with the SPA
# baked in, run `cd frontend && npm ci && npm run build` first, then build with
# `-tags dist`.
RUN CGO_ENABLED=0 go build -o /out/oci-start ./cmd/oci-start

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/oci-start /usr/local/bin/oci-start
COPY config.yaml /app/config.yaml
COPY migrations /app/migrations
EXPOSE 9856
ENV SERVER_PORT=9856
VOLUME ["/app/data", "/app/logs"]
ENTRYPOINT ["oci-start"]

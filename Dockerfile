# syntax=docker/dockerfile:1

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/bp-tracker ./cmd/server \
    && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/bp-tracker /bp-tracker
# Pre-create /data owned by the nonroot user so a fresh named volume inherits writable permissions.
COPY --from=server --chown=65532:65532 /out/data /data
ENV BP_ADDR=:8080 \
    BP_DB=/data/bp.db \
    BP_TZ=Asia/Shanghai
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/bp-tracker"]

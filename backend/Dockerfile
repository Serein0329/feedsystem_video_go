ARG GO_VERSION=1.26.1
ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.google.cn

FROM golang:${GO_VERSION} AS build
ARG GOPROXY
ARG GOSUMDB
ENV GOPROXY=${GOPROXY} \
  GOSUMDB=${GOSUMDB} \
  CGO_ENABLED=0

WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  go mod download

COPY backend/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  go build -trimpath -ldflags="-s -w" -o /out/api ./cmd

FROM alpine:3.21 AS api
RUN apk add --no-cache ca-certificates tzdata \
  && adduser -D -H -s /sbin/nologin app \
  && mkdir -p /app/.run/uploads \
  && chown -R app:app /app

WORKDIR /app
COPY --from=build /src/backend/configs ./configs
COPY --from=build /out/api ./api

USER app
EXPOSE 8080
ENTRYPOINT ["/app/api"]
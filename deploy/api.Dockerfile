FROM golang:1.26 AS build
ARG VERSION=dev
WORKDIR /src
COPY api/go.mod api/go.sum* ./
RUN go mod download
COPY api/ ./
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/api /usr/local/bin/api
USER nobody
EXPOSE 8080
ENTRYPOINT ["api"]

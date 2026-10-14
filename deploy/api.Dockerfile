FROM node:22-slim AS web
WORKDIR /src
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
# vite.config.ts bundles ../CHANGELOG.md for the changelog dialog.
COPY CHANGELOG.md /CHANGELOG.md
RUN pnpm build

FROM golang:1.26 AS build
ARG VERSION=dev
WORKDIR /src
COPY api/go.mod api/go.sum* ./
RUN go mod download
COPY api/ ./
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api

FROM debian:bookworm-slim
# pg_dump must be at least the server's major version, which is newer than
# Debian ships, so it comes from the PostgreSQL apt repository. Keep it in step
# with the postgres image in docker-compose.yml.
ARG PG_MAJOR=17
ADD https://www.postgresql.org/media/keys/ACCC4CF8.asc /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc
RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg ca-certificates \
    && chmod 644 /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc \
    && echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt bookworm-pgdg main" \
        > /etc/apt/sources.list.d/pgdg.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends postgresql-client-${PG_MAJOR} \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/api /usr/local/bin/api
COPY --from=web /src/dist /srv/web
ENV KRILL_WEB_DIR=/srv/web
USER nobody
EXPOSE 8080
ENTRYPOINT ["api"]

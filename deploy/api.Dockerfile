FROM golang:1.25 AS build
ARG VERSION=dev
WORKDIR /src
COPY api/go.mod api/go.sum* ./
RUN go mod download
COPY api/ ./
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]

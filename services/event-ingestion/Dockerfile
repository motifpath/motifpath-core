# Build context must be the monorepo root (motifpath-core/), not this directory:
#   docker build -f services/event-ingestion/Dockerfile -t event-ingestion .
#
# go.work declares both services as workspace members, so `go build` in workspace
# mode needs core-domain's directory to exist on disk too, even though only
# event-ingestion's binary is built here.

FROM golang:1.26.5-bookworm AS build
WORKDIR /src

COPY go.work go.work.sum ./
COPY services/event-ingestion/go.mod services/event-ingestion/go.sum services/event-ingestion/
COPY services/core-domain/go.mod services/core-domain/go.sum services/core-domain/
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -C services/event-ingestion -o /out/event-ingestion ./cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/event-ingestion /event-ingestion
USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["/event-ingestion"]

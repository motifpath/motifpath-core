# Build context must be the monorepo root (motifpath-core/), not this directory:
#   docker build -f services/aggregation-worker/Dockerfile -t aggregation-worker .
#
# go.work declares all three services as workspace members, so `go build` in
# workspace mode needs the other two services' directories to exist on disk
# too, even though only aggregation-worker's binary is built here.

FROM golang:1.26.5-bookworm AS build
WORKDIR /src

COPY go.work go.work.sum ./
COPY services/aggregation-worker/go.mod services/aggregation-worker/go.sum services/aggregation-worker/
COPY services/event-ingestion/go.mod services/event-ingestion/go.sum services/event-ingestion/
COPY services/core-domain/go.mod services/core-domain/go.sum services/core-domain/
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -C services/aggregation-worker -o /out/aggregation-worker ./cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/aggregation-worker /aggregation-worker
USER nonroot:nonroot
ENTRYPOINT ["/aggregation-worker"]

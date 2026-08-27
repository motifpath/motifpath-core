# Build context must be the monorepo root (motifpath-core/), not this directory:
#   docker build -f services/core-domain/Dockerfile -t core-domain .
#
# go.work declares all three services as workspace members, so `go build` in
# workspace mode needs every member's directory to exist on disk, even though
# only core-domain's binary is built here.

FROM golang:1.26.5-bookworm AS build
WORKDIR /src

COPY go.work go.work.sum ./
COPY services/core-domain/go.mod services/core-domain/go.sum services/core-domain/
COPY services/event-ingestion/go.mod services/event-ingestion/go.sum services/event-ingestion/
COPY services/aggregation-worker/go.mod services/aggregation-worker/go.sum services/aggregation-worker/
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -C services/core-domain -o /out/core-domain ./cmd

# Not distroless: cmd/main.go shells out to the Atlas CLI at startup to apply
# migrations under its Postgres advisory lock (ADR-005) — that needs a real
# base image to run a second binary in, which distroless/static doesn't have.
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
	&& rm -rf /var/lib/apt/lists/* \
	&& useradd --system --no-create-home --uid 65532 nonroot

COPY --from=arigaio/atlas:1.3.2 /atlas /usr/local/bin/atlas
COPY --from=build /out/core-domain /core-domain

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/core-domain"]

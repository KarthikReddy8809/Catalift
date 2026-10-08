# The server image: the API and the worker binaries with the channel and
# category files they read at start-up. Compose runs /api or /worker from
# the same image, so both always run the same commit.
FROM golang:1.26.8 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/worker ./cmd/worker \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/admin ./cmd/admin

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /out/worker /out/admin /
# CHANNELS_DIR and CATEGORIES_FILE default to config/... relative to the
# working directory; without these files the API refuses to start.
COPY --chown=nonroot:nonroot config /app/config
WORKDIR /app
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]

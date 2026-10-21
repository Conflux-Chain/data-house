# Build the service binary in a reproducible toolchain image.
FROM golang:1.23-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/data-house .

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home datahouse

COPY --from=build /out/data-house /usr/local/bin/data-house
USER datahouse
WORKDIR /home/datahouse

ENTRYPOINT ["/usr/local/bin/data-house"]

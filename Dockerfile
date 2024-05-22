FROM golang:bookworm AS builder

WORKDIR /app
COPY . .

RUN make

FROM debian:bookworm

WORKDIR /app
COPY --from=builder /app/hass-location-notifier /app/hass-location-notifier

WORKDIR /config

CMD ["/app/hass-location-notifier"]

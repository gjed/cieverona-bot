FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /cie-verona .

# -------------------------------------------------------
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /cie-verona /cie-verona
COPY calendars.json /calendars.json

WORKDIR /
VOLUME ["/data"]
ENTRYPOINT ["/cie-verona"]

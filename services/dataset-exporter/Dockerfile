FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /dataset-exporter .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /dataset-exporter /usr/local/bin/dataset-exporter
COPY migrations/ /migrations/

EXPOSE 9200
ENTRYPOINT ["dataset-exporter"]
CMD ["sync", "--migration", "/migrations/001_init.sql"]

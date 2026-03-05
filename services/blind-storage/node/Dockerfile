FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /storage-node .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /storage-node /usr/local/bin/storage-node

VOLUME /data/chunks
EXPOSE 9400
ENTRYPOINT ["storage-node"]
CMD ["serve"]

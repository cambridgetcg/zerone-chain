FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /api-gateway .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /api-gateway /usr/local/bin/api-gateway

EXPOSE 8080 9201
ENTRYPOINT ["api-gateway"]
CMD ["serve"]

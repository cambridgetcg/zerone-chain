FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /payment-bridge .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /payment-bridge /usr/local/bin/payment-bridge

EXPOSE 9300
ENTRYPOINT ["payment-bridge"]
CMD ["serve"]

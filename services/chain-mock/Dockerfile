FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /chain-mock .

FROM alpine:3.20
COPY --from=builder /chain-mock /chain-mock
EXPOSE 9090
ENTRYPOINT ["/chain-mock"]

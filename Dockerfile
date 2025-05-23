FROM golang:1.24-alpine AS builder
LABEL authors="MentalMentos"
WORKDIR /app
RUN apk add --no-cache git

COPY go.mod go.sum ./

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o lb ./cmd/app/main.go

FROM alpine:3.19
# CA сертификаты для http
RUN apk --no-cache add ca-certificates
WORKDIR /app

COPY --from=builder /app/lb /app/clients
COPY config.yaml .
EXPOSE 8080
ENTRYPOINT ["/app/clients"]
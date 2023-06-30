FROM golang:1.20-alpine3.16 AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY .env ./
COPY *.go ./

RUN go build -o subscriptionbot .
EXPOSE 8080

CMD ["./subscriptionbot"]



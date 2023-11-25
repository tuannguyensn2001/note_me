FROM golang:1.21 as builder

WORKDIR /app

EXPOSE 16000

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o main .


FROM alpine:3.16

WORKDIR /app

COPY --from=builder /app/main .

CMD ["./main"]

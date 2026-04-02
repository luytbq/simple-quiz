FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o quiz .

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/quiz .
EXPOSE 8080
VOLUME /data
ENV DB_PATH=/data/quiz.db
CMD ["./quiz"]

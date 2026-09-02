FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod ./
RUN go mod download 2>/dev/null; true
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o idmonitor-server ./main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o idmonitor-worker ./worker.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates nmap
WORKDIR /app
COPY --from=builder /app/idmonitor-server .
COPY --from=builder /app/idmonitor-worker .
EXPOSE 8080
CMD ["./idmonitor-server"]

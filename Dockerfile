FROM docker.io/library/golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /truthmetal ./cmd/server

FROM docker.io/library/alpine:3.19
COPY --from=builder /truthmetal /truthmetal
COPY --from=builder /app/migrations /migrations
ENV MIGRATIONS_PATH=file:///migrations
EXPOSE 8080 9090
ENTRYPOINT ["/truthmetal"]

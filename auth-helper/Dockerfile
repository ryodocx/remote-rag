FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod ./
# Add this so it doesn't fail if we don't have go.sum locally
RUN go mod download || true

COPY main.go ./
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o auth-helper main.go

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/auth-helper .

EXPOSE 8000
CMD ["./auth-helper"]

FROM golang:1.26.4-alpine AS builder

WORKDIR /app

COPY go.mod ./
# Add this so it doesn't fail if we don't have go.sum locally
RUN go mod download || true

COPY *.go ./
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o auth-helper main.go

FROM alpine:latest

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app
COPY --from=builder --chown=appuser:appgroup /app/auth-helper .

USER appuser

EXPOSE 8000
CMD ["./auth-helper"]

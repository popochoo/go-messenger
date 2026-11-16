FROM golang:1.27.1-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/cmd/gomessenger/exe ./cmd/gomessenger

FROM alpine:3.23
WORKDIR /app
COPY --from=builder /app/cmd/gomessenger/exe /app
COPY --from=builder /app/web /app/web
ENV PROJECT_ROOT=/app
CMD [ "/app/exe" ]
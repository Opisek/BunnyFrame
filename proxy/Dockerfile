# Build
FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum main.go ./
RUN go build

# Run
FROM alpine:latest
WORKDIR /app
EXPOSE 8080
ENTRYPOINT [ "/app/bunny-proxy" ]
COPY --from=build /app/BunFrame ./bunny-proxy
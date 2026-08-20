FROM golang:1.26.6-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /pulseboard ./cmd/server

FROM alpine:3.21
WORKDIR /app
RUN adduser -D -H appuser
USER appuser
COPY --from=build /pulseboard /pulseboard
COPY --from=build /app/docs ./docs
EXPOSE 8080
CMD ["/pulseboard"]

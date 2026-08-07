FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/server
FROM alpine:3.22
RUN adduser -D -u 10001 tappix
RUN mkdir -p /data/uploads && chown -R tappix:tappix /data/uploads
USER tappix
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]

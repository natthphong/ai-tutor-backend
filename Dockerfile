FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /toko-loop .
FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata ffmpeg curl && addgroup -g 10001 toko && adduser -D -u 10001 -G toko toko
ENV TZ=Asia/Bangkok AUDIO_DIR=/data/audio PORT=8080
WORKDIR /app
COPY --from=build /toko-loop /app/toko-loop
COPY config/models.yaml /app/models.yaml
ENV TOKO_CONFIG=/app/models.yaml
RUN mkdir -p /data/audio && chown -R toko:toko /data
USER toko
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=5s --start-period=20s CMD curl -fsS http://127.0.0.1:8080/ai-tutor/api/v2/readiness || exit 1
CMD ["/app/toko-loop"]

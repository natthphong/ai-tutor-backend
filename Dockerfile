FROM alpine:3.20

# Runtime deps:
#   tzdata          – Asia/Bangkok local time
#   ca-certificates – TLS for Gemini/OpenRouter/OpenAI/MinIO and yt-dlp
#   curl, coreutils – yt-dlp download / health checks / debugging
#   python3         – yt-dlp runs as a Python script
#   ffmpeg          – yt-dlp uses ffmpeg to extract/repackage audio tracks
#
# yt-dlp is used internally only for transcript generation — the backend
# downloads the YouTube audio locally, sends it to Gemini for STT +
# segmentation, then deletes the temp file. Playback always uses the
# YouTube embed URL, nothing is uploaded to MinIO.
RUN apk --no-cache add \
    tzdata \
    curl \
    coreutils \
    ca-certificates \
    python3 \
    ffmpeg

# Pull yt-dlp as a self-contained binary from the official release. Pinning
# to "latest" keeps us current with YouTube's extractor patches.
RUN curl -fSL https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp \
        -o /usr/local/bin/yt-dlp \
    && chmod +x /usr/local/bin/yt-dlp \
    && /usr/local/bin/yt-dlp --version

ENV TZ=Asia/Bangkok

WORKDIR /app

COPY ./goapp ./goapp
COPY ./sql ./sql

RUN chmod +x ./goapp

CMD ["./goapp"]


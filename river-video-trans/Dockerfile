FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o river-video-trans ./cmd/server

# BtbN static build includes NVENC + NVDEC (cuvid); Ubuntu's apt ffmpeg does not.
# Pinned to a stable *release-line* build (n9.0.x), NOT `master`/`master-latest`:
# master builds silently bump the required NVENC SDK and can out-run the host
# NVIDIA driver, which manifests as transcodes coming out as a green screen.
# Whenever this is bumped, re-verify on the GPU host that a real transcode
# produces correct frames (no green screen), not just that ffmpeg runs.
#
# CAVEAT: BtbN's dated `autobuild-*` tags are ephemeral — old ones get pruned
# upstream, at which point this URL 404s and the image fails to build (this has
# already bitten us once). There is no retained release-line URL from BtbN
# (only `master-latest`), so bumping to a current autobuild tag is a stopgap;
# the durable fix is to vendor a verified tarball somewhere we control.
# To bump: pick the newest release-line asset from
# https://github.com/BtbN/FFmpeg-Builds/releases and update both ARGs.
FROM alpine:3.21 AS ffmpeg-downloader
ARG FFMPEG_RELEASE=autobuild-2026-09-13-14-50
ARG FFMPEG_BUILD=ffmpeg-n9.0.1-29-gad500d59cb-linux64-gpl-9.0
RUN apk add --no-cache curl xz
RUN curl -fsSL -o /tmp/ffmpeg.tar.xz \
    https://github.com/BtbN/FFmpeg-Builds/releases/download/${FFMPEG_RELEASE}/${FFMPEG_BUILD}.tar.xz && \
    tar -xJf /tmp/ffmpeg.tar.xz --strip-components=2 -C /usr/local/bin \
        ${FFMPEG_BUILD}/bin/ffmpeg \
        ${FFMPEG_BUILD}/bin/ffprobe && \
    rm /tmp/ffmpeg.tar.xz

# nvidia/cuda runtime supplies libnvcuvid.so (NVDEC) and libnvidia-encode.so (NVENC)
# via nvidia-container-toolkit at runtime.
FROM nvidia/cuda:12.6.3-runtime-ubuntu24.04
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates tzdata && \
    rm -rf /var/lib/apt/lists/*
COPY --from=ffmpeg-downloader /usr/local/bin/ffmpeg  /usr/local/bin/ffmpeg
COPY --from=ffmpeg-downloader /usr/local/bin/ffprobe /usr/local/bin/ffprobe
WORKDIR /app
COPY --from=builder /app/river-video-trans .
CMD ["./river-video-trans"]

FROM golang:1.24 AS builder

RUN apt-get update && apt-get install -y \
    gnupg \
    ca-certificates \
    && apt-key update \
    && apt-get update && apt-get install -y \
    git \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./

# Download dependencies
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download


COPY . .


# Build the application
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
	make build


FROM ubuntu:22.04

RUN apt-get update && apt-get install -y \
    gnupg \
    ca-certificates \
    && apt-key update \
    && apt-get update && apt-get install -y \
    tzdata wget curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/kvstore .


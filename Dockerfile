# ─── Go automation service ────────────────────────────────────────────────────
# Build:  docker build --target automation -t automation .
# Run:    docker run -v /path/to/config.yaml:/config.yaml automation

FROM golang:1.25 AS go-builder

WORKDIR /build

# Copy module files and all local replace targets before downloading deps
COPY go.mod go.sum ./
COPY third_party/ third_party/

# Download the pre-built static FFmpeg library required by CGO.
# All three vendored tools share one ffmpeg-statigo copy (see go.mod replace directive).
RUN cd third_party/linuxmatters/jivefire/third_party/ffmpeg-statigo && \
    go run ./cmd/download-lib

# Cache Go module dependencies
RUN go mod download

# Copy application source
COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=1 GOOS=linux \
    go build -o /automation ./cmd/automation


FROM debian:bookworm-slim AS automation

# libstdc++6: required at runtime (ffmpeg-statigo links -lstdc++ dynamically)
# ca-certificates: required for cover art HTTPS downloads
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libstdc++6 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=go-builder /automation /automation

ENTRYPOINT ["/automation"]
CMD ["--config", "/config.yaml"]


# ─── Next.js dashboard ────────────────────────────────────────────────────────
# Build:  docker build --target dashboard -t dashboard .
# Run:    docker run -e API_TARGETS=prod=http://prod:8080 -p 3000:3000 dashboard

FROM node:22-alpine AS dashboard-deps

WORKDIR /dashboard
COPY dashboard/package.json dashboard/package-lock.json* ./
RUN npm install --frozen-lockfile 2>/dev/null || npm install


FROM node:22-alpine AS dashboard-builder

WORKDIR /dashboard
ENV NEXT_TELEMETRY_DISABLED=1

COPY --from=dashboard-deps /dashboard/node_modules ./node_modules
COPY dashboard/ .

RUN npm run build


FROM node:22-alpine AS dashboard

WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
# Listen on all interfaces inside Docker
ENV HOSTNAME=0.0.0.0
ENV PORT=3000

COPY --from=dashboard-builder /dashboard/.next/standalone ./
COPY --from=dashboard-builder /dashboard/.next/static ./.next/static
COPY --from=dashboard-builder /dashboard/public ./public

EXPOSE 3000
CMD ["node", "server.js"]

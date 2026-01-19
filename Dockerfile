# ═══════════════════════════════════════════════════════════
# 1. Stage: Go Builder
# ═══════════════════════════════════════════════════════════
FROM golang:1.24-bookworm AS go-builder

RUN apt-get update && apt-get install -y \
    gcc \
    libc6-dev \
    git \
    libsqlite3-dev \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY . .

# پرانی فائلز ہٹائیں
RUN rm -f go.mod go.sum || true

# فریش لائبریریز ڈاؤن لوڈ کریں
RUN go mod init impossible-bot && \
    go get go.mau.fi/whatsmeow@latest && \
    go get go.mongodb.org/mongo-driver/mongo@latest && \
    go get go.mongodb.org/mongo-driver/bson@latest && \
    go get github.com/redis/go-redis/v9@latest && \
    go get github.com/gin-gonic/gin@latest && \
    go get github.com/mattn/go-sqlite3@latest && \
    go get github.com/lib/pq@latest && \
    go get github.com/gorilla/websocket@latest && \
    go get google.golang.org/protobuf/proto@latest && \
    go get github.com/showwin/speedtest-go && \
    go get google.golang.org/genai && \
    go mod tidy

RUN CGO_ENABLED=1 GOOS=linux go build -v -ldflags="-s -w" -o bot .

# ═══════════════════════════════════════════════════════════
# 2. Stage: Node.js Builder
# ═══════════════════════════════════════════════════════════
FROM node:20-bookworm-slim AS node-builder
RUN apt-get update && apt-get install -y git && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY package*.json ./
COPY lid-extractor.js ./
RUN npm install --production

# ═══════════════════════════════════════════════════════════
# 3. Stage: Final Runtime (FIXED FOR TTS)
# ═══════════════════════════════════════════════════════════
# 👇 تبدیلی: Python 3.12 کی جگہ 3.10 (کیونکہ TTS 3.12 پر نہیں چلتا)
FROM python:3.10-slim-bookworm

# ✅ سسٹم ٹولز (espeak-ng ایڈ کیا ہے جو TTS کے لیے لازمی ہے)
RUN apt-get update && apt-get install -y \
    ffmpeg \
    imagemagick \
    curl \
    sqlite3 \
    libsqlite3-0 \
    nodejs \
    npm \
    ca-certificates \
    libgomp1 \
    megatools \
    libwebp-dev \
    webp \
    libwebpmux3 \
    libwebpdemux2 \
    libsndfile1 \
    espeak-ng \
    && rm -rf /var/lib/apt/lists/*

# YT-DLP انسٹال کریں
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp \
    && chmod a+rx /usr/local/bin/yt-dlp

# ✅ Python AI Libraries
# نوٹ: TTS کو بڑے حروف (Capital) میں لکھا ہے اور کچھ ورژنز فکس کیے ہیں
RUN pip3 install --no-cache-dir \
    onnxruntime \
    rembg[cli] \
    fastapi \
    uvicorn \
    python-multipart \
    requests \
    faster-whisper \
    TTS \
    scipy

# ✅ Coqui TTS لائسنس
ENV COQUI_TOS_AGREED=1

WORKDIR /app

# پرانی سٹیجز سے فائلیں کاپی کریں
COPY --from=go-builder /app/bot ./bot
COPY --from=node-builder /app/node_modules ./node_modules
COPY --from=node-builder /app/lid-extractor.js ./lid-extractor.js
COPY --from=node-builder /app/package.json ./package.json

# لوکل فائلیں کاپی کریں
COPY web ./web
COPY pic.png ./pic.png
COPY ai_engine.py ./ai_engine.py
COPY voices ./voices

RUN mkdir -p store logs
ENV PORT=8080
ENV NODE_ENV=production
ENV U2NET_HOME=/app/store/.u2net 
EXPOSE 8080

# بوٹ چلائیں
CMD ["/app/bot"]

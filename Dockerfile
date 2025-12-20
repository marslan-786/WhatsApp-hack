# ═══════════════════════════════════════════════════════════
# 1. Stage: Go Builder
# ═══════════════════════════════════════════════════════════
FROM golang:1.24-alpine AS go-builder

# انسٹال ٹولز (ffmpeg-dev بھی شامل ہے)
RUN apk add --no-cache gcc musl-dev git sqlite-dev ffmpeg-dev

WORKDIR /app

# کوڈ کاپی کریں
COPY . .

# پرانی فائلز کی صفائی (تاکہ کنفلکٹ نہ ہو)
RUN rm -f go.mod go.sum || true

# ماڈیول شروع کریں اور لائبریریز کھینچیں
RUN go mod init impossible-bot && \
    go get go.mau.fi/whatsmeow@latest && \
    go get go.mongodb.org/mongo-driver/mongo@latest && \
    go get go.mongodb.org/mongo-driver/bson@latest && \
    go get github.com/gin-gonic/gin@latest && \
    go get github.com/mattn/go-sqlite3@latest && \
    go get github.com/lib/pq@latest && \
    go get github.com/gorilla/websocket@latest && \
    go get google.golang.org/protobuf/proto@latest && \
    go mod tidy

# بوٹ کو کمپائل کریں
RUN go build -ldflags="-s -w" -o bot .

# ═══════════════════════════════════════════════════════════
# 2. Stage: Node.js Builder (LID سسٹم کے لئے)
# ═══════════════════════════════════════════════════════════
FROM node:20-alpine AS node-builder
RUN apk add --no-cache git 

WORKDIR /app

# صرف ضروری فائلز کاپی کریں
COPY package*.json ./
COPY lid-extractor.js ./

# نوڈ کے پیکجز انسٹال کریں
RUN npm install --production

# ═══════════════════════════════════════════════════════════
# 3. Stage: Final Runtime (اصل چلنے والی امیج)
# ═══════════════════════════════════════════════════════════
FROM alpine:latest

# رن ٹائم ڈیپینڈینسیز (Go کے لئے اور Node کے لئے بھی)
RUN apk add --no-cache \
    ca-certificates \
    sqlite-libs \
    ffmpeg \
    nodejs \
    npm \
    && rm -rf /var/cache/apk/*

WORKDIR /app

# Go کا تیار شدہ بوٹ اٹھائیں
COPY --from=go-builder /app/bot ./bot

# Node.js کا تیار شدہ فولڈر اور اسکرپٹ اٹھائیں
COPY --from=node-builder /app/node_modules ./node_modules
COPY --from=node-builder /app/lid-extractor.js ./lid-extractor.js
COPY --from=node-builder /app/package.json ./package.json

# باقی اثاثے (Assets) کاپی کریں
COPY web ./web
COPY pic.png ./pic.png

# فولڈرز بنائیں
RUN mkdir -p store logs

# پورٹ اور انوائرمنٹ
ENV PORT=8080
ENV NODE_ENV=production
EXPOSE 8080

# بوٹ اسٹارٹ کریں
CMD ["./bot"]

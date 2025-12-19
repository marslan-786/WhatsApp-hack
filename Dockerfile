FROM golang:1.24-alpine AS builder

# ضروری سسٹم پیکجز بشمول FFmpeg
RUN apk add --no-cache gcc musl-dev git sqlite-dev ffmpeg-dev

WORKDIR /app
COPY . .

# گو موڈ کو زیرو سے شروع کرنا (آپ کو فائل بنانے کی ضرورت نہیں)
RUN rm -f go.mod go.sum || true
RUN go mod init impossible-bot

# تمام لائبریریز کے تازہ ترین ورژن حاصل کرنا
RUN go get go.mau.fi/whatsmeow@latest
RUN go get go.mongodb.org/mongo-driver/mongo@latest
RUN go get github.com/gin-gonic/gin@latest
RUN go get github.com/mattn/go-sqlite3@latest
RUN go get github.com/lib/pq@latest
RUN go mod tidy

# بوٹ بلڈ کرنا
RUN go build -o bot .

# رن ٹائم سٹیج
FROM alpine:latest
RUN apk add --no-cache ca-certificates sqlite-libs ffmpeg

WORKDIR /app
COPY --from=builder /app/bot .
COPY --from=builder /app/web ./web

EXPOSE 8080
CMD ["./bot"]
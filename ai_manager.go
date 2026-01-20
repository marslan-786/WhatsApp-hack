package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// 🧠 UNIVERSAL SESSION STRUCTURE
type AISession struct {
	History     string   `json:"history"`       // چیٹ کی پوری کہانی
	MessageIDs  []string `json:"message_ids"`   // پچھلے 100 میسجز کی IDs
	LastUpdated int64    `json:"last_updated"`
}

// ✅ 1. CHECK IF REPLY IS TO AI (Any of last 100 messages)
func IsReplyToAI(senderID string, replyID string) bool {
	if rdb == nil {
		return false
	}

	ctx := context.Background()
	val, err := rdb.Get(ctx, "ai_session:"+senderID).Result()
	if err != nil {
		return false
	}

	var session AISession
	json.Unmarshal([]byte(val), &session)

	// 🔍 پچھلے 100 میسجز میں چیک کریں
	for _, id := range session.MessageIDs {
		if id == replyID {
			return true // میچ مل گیا!
		}
	}
	return false
}

// ✅ 2. GET HISTORY (Text + Voice Combined)
func GetAIHistory(senderID string) string {
	if rdb == nil {
		return ""
	}
	ctx := context.Background()
	val, err := rdb.Get(ctx, "ai_session:"+senderID).Result()
	if err == nil {
		var session AISession
		json.Unmarshal([]byte(val), &session)
		// 1 گھنٹے تک یاد رکھے (3600 سیکنڈز)
		if time.Now().Unix()-session.LastUpdated < 3600 {
			return session.History
		}
	}
	return ""
}

// ✅ 3. SAVE HISTORY (Universal Update)
func SaveAIHistory(senderID string, userQuery string, aiResponse string, newMsgID string) {
	if rdb == nil {
		return
	}
	ctx := context.Background()
	key := "ai_session:" + senderID

	// پرانا ڈیٹا اٹھائیں
	var session AISession
	val, err := rdb.Get(ctx, key).Result()
	if err == nil {
		json.Unmarshal([]byte(val), &session)
	}

	// 📝 History Update
	newEntry := fmt.Sprintf("\nUser: %s\nAI: %s", userQuery, aiResponse)
	session.History += newEntry

	// ہسٹری زیادہ لمبی نہ ہو (Max 2000 chars - تقریباً 300 الفاظ)
	if len(session.History) > 2000 {
		session.History = session.History[len(session.History)-2000:]
	}

	// 🆔 Message ID Tracking (Last 100)
	if newMsgID != "" {
		session.MessageIDs = append(session.MessageIDs, newMsgID)
		// اگر 100 سے زیادہ ہو جائیں تو پرانے ڈیلیٹ کر دیں (FIFO)
		if len(session.MessageIDs) > 100 {
			session.MessageIDs = session.MessageIDs[len(session.MessageIDs)-100:]
		}
	}

	session.LastUpdated = time.Now().Unix()

	// Redis میں سیو کریں (1 گھنٹے کا ٹائم آؤٹ)
	jsonData, _ := json.Marshal(session)
	rdb.Set(ctx, key, jsonData, 60*time.Minute)
}
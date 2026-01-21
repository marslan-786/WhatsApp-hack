package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/genai"
	"google.golang.org/protobuf/proto"
)

// 💾 Redis Keys
const (
	KeyAutoAITarget = "autoai:target_user"  
	KeyChatHistory  = "chat:history:%s:%s" // botID:chatID -> History
	KeyLastMsgTime  = "autoai:last_msg_time:%s" // chatID -> Timestamp
	KeyLastOwnerMsg = "autoai:last_owner_msg:%s" // chatID -> Timestamp
)

// 📝 1. HISTORY RECORDER (Only Personal Chats)
func RecordChatHistory(client *whatsmeow.Client, v *events.Message, botID string) {
	// Ignore Groups & Channels
	if v.Info.IsGroup || strings.Contains(v.Info.Chat.String(), "@newsletter") || v.Info.Chat.String() == "status@broadcast" {
		return
	}

	ctx := context.Background()
	chatID := v.Info.Chat.String()

	// 🕒 اگر یہ میرا (Owner) میسج ہے تو ٹائم نوٹ کر لیں (تاکہ AI کو روکا جا سکے)
	if v.Info.IsFromMe {
		rdb.Set(ctx, fmt.Sprintf(KeyLastOwnerMsg, chatID), time.Now().Unix(), 0)
	}

	// Ignore Junk Media (Video, Sticker, etc - except Audio)
	if v.Message.GetVideoMessage() != nil || 
	   v.Message.GetStickerMessage() != nil || 
	   v.Message.GetDocumentMessage() != nil {
		return
	}

	senderName := v.Info.PushName
	if v.Info.IsFromMe {
		senderName = "Me"
	} else if senderName == "" {
		if contact, err := client.Store.Contacts.GetContact(ctx, v.Info.Sender); err == nil && contact.Found {
			senderName = contact.FullName
		}
		if senderName == "" { senderName = "User" }
	}

	text := ""
	if v.Message.GetAudioMessage() != nil {
		text = "[Voice Message]" 
	} else {
		text = v.Message.GetConversation()
		if text == "" {
			text = v.Message.GetExtendedTextMessage().GetText()
		}
	}

	if text == "" { return }

	// 💾 Save History
	entry := fmt.Sprintf("%s: %s", senderName, text)
	key := fmt.Sprintf(KeyChatHistory, botID, chatID)
	rdb.RPush(ctx, key, entry)
	rdb.LTrim(ctx, key, -50, -1) // Keep last 50
}

// 🚀 2. COMMAND HANDLER
func HandleAutoAICmd(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) == 0 {
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "⚠️ Usage: .autoai set <Name>")
		return
	}

	mode := strings.ToLower(args[0])
	ctx := context.Background()

	switch mode {
	case "set":
		if len(args) < 2 {
			sendCleanReply(client, v.Info.Chat, v.Info.ID, "❌ Name required.")
			return
		}
		targetName := strings.Join(args[1:], " ")
		rdb.Set(ctx, KeyAutoAITarget, targetName, 0)
		fmt.Printf("\n🔥 [AUTO-AI] TARGET LOCKED: %s\n", targetName)
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "✅ AI Set on: "+targetName)

	case "off":
		rdb.Del(ctx, KeyAutoAITarget)
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "🛑 Auto AI Stopped.")
	}
}

// 🧠 3. MAIN CHECKER
func CheckAndHandleAutoReply(client *whatsmeow.Client, v *events.Message) bool {
	if v.Info.IsFromMe { return false }

	ctx := context.Background()
	targetName, err := rdb.Get(ctx, KeyAutoAITarget).Result()
	if err != nil || targetName == "" { return false }

	incomingName := v.Info.PushName
	if incomingName == "" {
		if contact, err := client.Store.Contacts.GetContact(ctx, v.Info.Sender); err == nil && contact.Found {
			incomingName = contact.FullName
			if incomingName == "" { incomingName = contact.PushName }
		}
	}

	if strings.Contains(strings.ToLower(incomingName), strings.ToLower(targetName)) {
		fmt.Printf("🔔 [AI MATCH] Chatting with %s\n", incomingName)
		go processAIResponse(client, v, incomingName)
		return true 
	}
	return false
}

// 🤖 4. AI BEHAVIOR ENGINE (Human Logic)
func processAIResponse(client *whatsmeow.Client, v *events.Message, senderName string) {
	ctx := context.Background()
	chatID := v.Info.Chat.String()
	
	// ⏳ A. CHECK TIMING (Active vs Cold)
	lastTimeStr, _ := rdb.Get(ctx, fmt.Sprintf(KeyLastMsgTime, chatID)).Result()
	var lastTime int64
	if lastTimeStr != "" {
		fmt.Sscanf(lastTimeStr, "%d", &lastTime)
	}
	
	// Update Last Msg Time
	currentTime := time.Now().Unix()
	rdb.Set(ctx, fmt.Sprintf(KeyLastMsgTime, chatID), fmt.Sprintf("%d", currentTime), 0)

	timeDiff := currentTime - lastTime
	isActiveChat := timeDiff < 60 // Less than 1 min = Active

	// 🛑 B. COLD START LOGIC (Wait + Fake Typing)
	if !isActiveChat {
		fmt.Printf("🐢 [MODE] Cold Start (Gap: %d sec). Waiting 10s...\n", timeDiff)
		
		// 1. Wait to "Pick up phone"
		time.Sleep(10 * time.Second)
		
		// 2. Online
		// ✅ FIX: Removed extra arguments
		client.SendPresence(ctx, types.PresenceAvailable)

		// 3. Fake Typing Loop (Check for Owner Interruption)
		typingDuration := 30 // 30 seconds wait
		fmt.Println("✍️ [AI] Fake Typing / Waiting for Owner...")
		
		for i := 0; i < typingDuration; i++ {
			// Check if owner sent a message recently
			lastOwnerMsgStr, _ := rdb.Get(ctx, fmt.Sprintf(KeyLastOwnerMsg, chatID)).Result()
			var lastOwnerMsg int64
			if lastOwnerMsgStr != "" { fmt.Sscanf(lastOwnerMsgStr, "%d", &lastOwnerMsg) }

			// If owner replied AFTER the user message came in -> ABORT
			if lastOwnerMsg > v.Info.Timestamp.Unix() {
				fmt.Println("🛑 [AI ABORT] Owner took over!")
				client.SendChatPresence(ctx, v.Info.Chat, types.ChatPresencePaused, types.ChatPresenceMediaText)
				return 
			}

			// Show Typing every 5 seconds
			if i%5 == 0 {
				client.SendChatPresence(ctx, v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)
			}
			time.Sleep(1 * time.Second)
		}
	} else {
		// ⚡ ACTIVE CHAT: No waiting, Instant Online
		fmt.Println("⚡ [MODE] Active Chat! Instant Reply.")
		// ✅ FIX: Removed extra arguments
		client.SendPresence(ctx, types.PresenceAvailable)
	}

	// 🛑 FINAL OWNER CHECK
	lastOwnerMsgStr, _ := rdb.Get(ctx, fmt.Sprintf(KeyLastOwnerMsg, chatID)).Result()
	var lastOwnerMsg int64
	if lastOwnerMsgStr != "" { fmt.Sscanf(lastOwnerMsgStr, "%d", &lastOwnerMsg) }
	if lastOwnerMsg > v.Info.Timestamp.Unix() {
		fmt.Println("🛑 [AI ABORT] Owner replied at last second!")
		return 
	}

	// 📥 C. PROCESS INPUT (Text / Voice)
	userText := ""
	isVoice := false
	voiceDuration := 0

	if v.Message.GetAudioMessage() != nil {
		isVoice = true
		voiceDuration = int(v.Message.GetAudioMessage().GetSeconds())
		if voiceDuration == 0 { voiceDuration = 5 }
		
		fmt.Printf("🎤 [VOICE] Listening... (%d sec)\n", voiceDuration)
		
		// ✅ 1. Mark Read (Blue Tick)
		client.MarkRead(ctx, []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender)
		
		// ✅ 2. Simulate "Playing" (Listening Time)
		// بلیو ٹک کے فوراً بعد پلے والا سین (ہم اسے صرف ٹائمنگ سے ہینڈل کر رہے ہیں)
		time.Sleep(time.Duration(voiceDuration) * time.Second)
		
		// ✅ 3. Transcribe
		data, err := client.Download(ctx, v.Message.GetAudioMessage())
		if err == nil {
			userText, _ = TranscribeAudio(data)
			userText = "[Voice Message]: " + userText
		}
	} else {
		userText = v.Message.GetConversation()
		if userText == "" { userText = v.Message.GetExtendedTextMessage().GetText() }
		
		// Text Reading Time
		if userText != "" {
			// ✅ Mark Read Immediately
			client.MarkRead(ctx, []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender)
			
			readDelay := len(userText) / 10
			if readDelay < 2 { readDelay = 2 }
			time.Sleep(time.Duration(readDelay) * time.Second)
		}
	}

	if userText == "" { return }

	// 🧠 D. GENERATE REPLY
	rawBotID := client.Store.ID.User
	botID := strings.Split(rawBotID, ":")[0]
	botID = strings.Split(botID, "@")[0]

	aiResponse := generateCloneReply(botID, chatID, userText, senderName)
	if aiResponse == "" { return }

	// ✍️ E. REAL TYPING
	client.SendChatPresence(ctx, v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	
	typeSpeed := len(aiResponse) / 8
	if isActiveChat { typeSpeed = len(aiResponse) / 15 } // Active chat = faster typing
	if typeSpeed < 2 { typeSpeed = 2 }
	
	time.Sleep(time.Duration(typeSpeed) * time.Second)

	client.SendChatPresence(ctx, v.Info.Chat, types.ChatPresencePaused, types.ChatPresenceMediaText)
	sendCleanReply(client, v.Info.Chat, v.Info.ID, aiResponse)
	
	// Save AI Reply
	key := fmt.Sprintf(KeyChatHistory, botID, chatID)
	rdb.RPush(ctx, key, "Me: "+aiResponse)
	
	fmt.Printf("🚀 [SENT] %s\n", aiResponse)
	
	// 👋 F. GO OFFLINE (After 10s inactivity)
	go func() {
		time.Sleep(10 * time.Second)
		// ✅ FIX: Removed extra arguments
		client.SendPresence(context.Background(), types.PresenceUnavailable)
	}()
}

// 🧬 CLONE ENGINE (STRICT PERSONA)
func generateCloneReply(botID, chatID, currentMsg, senderName string) string {
	ctx := context.Background()
	historyList, _ := rdb.LRange(ctx, fmt.Sprintf(KeyChatHistory, botID, chatID), 0, -1).Result()
	history := strings.Join(historyList, "\n")

	// 🔥🔥🔥 DYNAMIC PROMPT 🔥🔥🔥
	fullPrompt := fmt.Sprintf(`
You are "Me" (The Owner). You are chatting with "%s".
Your goal is to reply EXACTLY like "Me" based on the history below.

CRITICAL INSTRUCTIONS:
1. **ANALYZE HISTORY:** Look at how "Me" replies. Short? Long? Funny? Rude? COPY THAT VIBE.
2. **IGNORE HARD RULES:** Do not follow generic AI rules. If "Me" is rude in history, be rude. If "Me" is sweet, be sweet.
3. **CLOSER:** Try to end the conversation naturally if "Me" usually does that. Don't ask new questions unless necessary.
4. **NO ROBOTIC TALK:** Never say "How can I help?". Say "Han g", "Bol", "Acha" etc.
5. **VOICE:** If input is [Voice Message], reply to what you think they said contextually.

CHAT HISTORY:
%s
---
USER: %s
ME:`, senderName, history, currentMsg)

	var keys []string
	if k := os.Getenv("GOOGLE_API_KEY"); k != "" { keys = append(keys, k) }
	for i := 1; i <= 50; i++ {
		if k := os.Getenv(fmt.Sprintf("GOOGLE_API_KEY_%d", i)); k != "" { keys = append(keys, k) }
	}

	for _, key := range keys {
		client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: key})
		if err != nil { continue }
		resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash", genai.Text(fullPrompt), nil)
		if err == nil { return resp.Text() }
	}
	return ""
}

func sendCleanReply(client *whatsmeow.Client, chat types.JID, replyToID string, text string) {
	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{StanzaID: proto.String(replyToID), Participant: proto.String(chat.String())},
		},
	}
	client.SendMessage(context.Background(), chat, msg)
}
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
)

// 📝 1. HISTORY RECORDER (Only Personal Chats)
func RecordChatHistory(client *whatsmeow.Client, v *events.Message, botID string) {
	// Ignore Groups & Channels
	if v.Info.IsGroup || strings.Contains(v.Info.Chat.String(), "@newsletter") || v.Info.Chat.String() == "status@broadcast" {
		return
	}

	// Ignore Junk Media (Video, Sticker, etc - except Audio)
	if v.Message.GetVideoMessage() != nil || 
	   v.Message.GetStickerMessage() != nil || 
	   v.Message.GetDocumentMessage() != nil {
		return
	}

	ctx := context.Background()
	chatID := v.Info.Chat.String()
	
	senderName := v.Info.PushName
	if v.Info.IsFromMe {
		senderName = "Me" // Owner
	} else if senderName == "" {
		if contact, err := client.Store.Contacts.GetContact(ctx, v.Info.Sender); err == nil && contact.Found {
			senderName = contact.FullName
		}
		if senderName == "" { senderName = "User" }
	}

	// Text Extraction
	text := ""
	if v.Message.GetAudioMessage() != nil {
		// Voice handled later or tagged
		text = "[Voice Message]" 
		// Note: We record it as a tag. The AI processor will transcribe real-time.
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

	// Name Matching
	incomingName := v.Info.PushName
	if incomingName == "" {
		if contact, err := client.Store.Contacts.GetContact(ctx, v.Info.Sender); err == nil && contact.Found {
			incomingName = contact.FullName
			if incomingName == "" { incomingName = contact.PushName }
		}
	}

	// Case Insensitive Match
	if strings.Contains(strings.ToLower(incomingName), strings.ToLower(targetName)) {
		fmt.Printf("🔔 [AI MATCH] Chatting with %s\n", incomingName)
		go processAIResponse(client, v, incomingName)
		return true 
	}
	return false
}

// 🤖 4. AI BEHAVIOR ENGINE (The Human Simulator)
func processAIResponse(client *whatsmeow.Client, v *events.Message, senderName string) {
	ctx := context.Background()
	chatID := v.Info.Chat.String()
	
	// ⏳ A. TIMING LOGIC (Cold vs Warm)
	lastTimeStr, _ := rdb.Get(ctx, fmt.Sprintf(KeyLastMsgTime, chatID)).Result()
	var lastTime int64
	if lastTimeStr != "" {
		fmt.Sscanf(lastTimeStr, "%d", &lastTime)
	}
	currentTime := time.Now().Unix()
	timeDiff := currentTime - lastTime
	
	// Update Last Time
	rdb.Set(ctx, fmt.Sprintf(KeyLastMsgTime, chatID), fmt.Sprintf("%d", currentTime), 0)

	isActiveChat := timeDiff < 60 // Less than 1 min gap
	waitSec := 0

	if isActiveChat {
		fmt.Println("⚡ [MODE] Active Chat (Instant Read)")
		waitSec = 0 // Instant
	} else {
		fmt.Printf("🐢 [MODE] Cold Start (Gap: %d sec). Picking up phone...\n", timeDiff)
		waitSec = 8 + rand.Intn(5) // 8-12 seconds to "pick up phone"
	}

	// 💤 Sleep for "Pick up" time
	if waitSec > 0 {
		time.Sleep(time.Duration(waitSec) * time.Second)
	}

	// 🟢 B. ONLINE STATUS
	client.SendPresence(ctx, v.Info.Chat, types.PresenceAvailable) // Show Online

	// 🛑 C. OWNER PRIORITY WAIT (30-60 Seconds)
	// اگر یہ کولڈ سٹارٹ تھا، تو ہو سکتا ہے اونر خود دیکھ لے
	// اگر ایکٹیو چیٹ ہے تو ہم جلدی جواب دیں گے
	ownerWait := 40 
	if isActiveChat { ownerWait = 5 } // اگر چیٹ چل رہی ہے تو اونر کو صرف 5 سیکنڈ دیں

	fmt.Printf("⏳ Waiting %d sec for Owner to reply...\n", ownerWait)
	
	// اس دوران ہم "Online" نظر آئیں گے لیکن بلیو ٹک نہیں دیں گے (جیسے چیٹ لسٹ دیکھ رہے ہیں)
	for i := 0; i < ownerWait; i++ {
		time.Sleep(1 * time.Second)
		// TODO: Check here if owner sent a message (requires DB check logic, skipping for simplicity to trust the AI takeover)
	}

	// 📥 D. READ INPUT (Text or Voice)
	userText := ""
	isVoice := false
	voiceDuration := 0

	if v.Message.GetAudioMessage() != nil {
		isVoice = true
		voiceDuration = int(v.Message.GetAudioMessage().GetSeconds())
		if voiceDuration == 0 { voiceDuration = 5 } // Fallback
		
		fmt.Printf("🎤 [VOICE] Listening... (%d sec)\n", voiceDuration)
		
		// 🎧 SIMULATE LISTENING
		// پہلے بلیو ٹک نہیں دینا، پہلے سننا ہے
		// سننے کے دوران "Online" رہنا ہے
		client.SendPresence(ctx, v.Info.Chat, types.PresenceAvailable)
		
		// سننے کا ٹائم گزاریں
		time.Sleep(time.Duration(voiceDuration) * time.Second)
		
		// Transcribe
		data, err := client.Download(ctx, v.Message.GetAudioMessage())
		if err == nil {
			userText, _ = TranscribeAudio(data)
			userText = "[Voice Message]: " + userText
		}
	} else {
		userText = v.Message.GetConversation()
		if userText == "" { userText = v.Message.GetExtendedTextMessage().GetText() }
	}

	if userText == "" { return }

	// ✅ E. BLUE TICK (Mark Read / Played)
	// اب ہم نے پڑھ لیا / سن لیا
	fmt.Println("👀 [READ] Marking Blue Tick")
	client.MarkRead(ctx, []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender)
	
	// Thinking Delay (Reading the text)
	if !isVoice {
		readDelay := len(userText) / 10
		if readDelay < 2 { readDelay = 2 }
		time.Sleep(time.Duration(readDelay) * time.Second)
	}

	// 🧠 F. GENERATE REPLY
	rawBotID := client.Store.ID.User
	botID := strings.Split(rawBotID, ":")[0]
	botID = strings.Split(botID, "@")[0]

	// جذبات چیک کریں اور ری ایکٹ کریں (Reaction)
	go attemptReaction(client, v, userText)

	aiResponse := generateCloneReply(botID, chatID, userText, senderName)
	if aiResponse == "" { return }

	// ✍️ G. TYPING & SENDING
	client.SendChatPresence(ctx, v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	
	// ٹائپنگ سپیڈ (Human like)
	typingTime := len(aiResponse) / 8 // تھوڑا سلو ٹائپنگ تاکہ اصلی لگے
	if typingTime < 2 { typingTime = 2 }
	time.Sleep(time.Duration(typingTime) * time.Second)

	client.SendChatPresence(ctx, v.Info.Chat, types.ChatPresencePaused, types.ChatPresenceMediaText)
	sendCleanReply(client, v.Info.Chat, v.Info.ID, aiResponse)
	
	// Save AI Reply to History
	key := fmt.Sprintf(KeyChatHistory, botID, chatID)
	rdb.RPush(ctx, key, "Me: "+aiResponse)
	
	fmt.Printf("🚀 [SENT] %s\n", aiResponse)
	
	// 👋 H. GO OFFLINE (If inactive)
	// میسج بھیجنے کے بعد 10 سیکنڈ تک آن لائن رہیں، پھر بھاگ جائیں
	time.Sleep(10 * time.Second)
	client.SendPresence(ctx, v.Info.Chat, types.PresenceUnavailable)
}

// 😄 Reaction Logic
func attemptReaction(client *whatsmeow.Client, v *events.Message, text string) {
	text = strings.ToLower(text)
	reaction := ""
	
	// Simple Keyword Analysis for Speed
	if strings.Contains(text, "haha") || strings.Contains(text, "lol") || strings.Contains(text, "funny") {
		reaction = "😂"
	} else if strings.Contains(text, "sad") || strings.Contains(text, "dukh") || strings.Contains(text, "ron") {
		reaction = "🥺"
	} else if strings.Contains(text, "love") || strings.Contains(text, "pyar") {
		reaction = "❤️"
	}

	if reaction != "" {
		// تھوڑا رک کر ری ایکٹ کریں
		time.Sleep(2 * time.Second)
		client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ReactionMessage: &waProto.ReactionMessage{
				Key: &waProto.MessageKey{
					RemoteJID: proto.String(v.Info.Chat.String()),
					ID:        proto.String(v.Info.ID),
					FromMe:    proto.Bool(false),
				},
				Text: proto.String(reaction),
				SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
			},
		})
	}
}

// 🧬 CLONE ENGINE (STRICT PERSONA)
func generateCloneReply(botID, chatID, currentMsg, senderName string) string {
	ctx := context.Background()
	historyList, _ := rdb.LRange(ctx, fmt.Sprintf(KeyChatHistory, botID, chatID), 0, -1).Result()
	history := strings.Join(historyList, "\n")

	// 🔥🔥🔥 THE ULTIMATE PROMPT 🔥🔥🔥
	fullPrompt := fmt.Sprintf(`
You are Arslan (The Owner). You are chatting with "%s".
You are NOT an AI. NEVER admit you are an AI. If asked, make a joke.

YOUR STYLE:
1. **Brief & Dismissive:** Do not write long paragraphs. Use 1-2 sentences.
2. **Conversation Closer:** Try to close the topic nicely. Don't ask open-ended questions like "Aur sunao" unless necessary.
3. **Natural Flow:** If they say "Hi", just say "Hi" or "Han g". Don't say "How can I help you?".
4. **Emoji:** Mirror the user. If they use emojis, use 1. If not, use none.
5. **Voice Replies:** If input is [Voice Message], reply to the content naturally.

PAST HISTORY (Copy this tone):
%s
---
USER: %s
ARSLAN:`, senderName, history, currentMsg)

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
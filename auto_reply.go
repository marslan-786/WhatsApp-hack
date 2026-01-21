package main

import (
	"context"
	"fmt"
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
	KeyAutoAIPrompt = "autoai:custom_prompt" 
	KeyLastMsgTime  = "autoai:last_msg_time" 
	KeyChatHistory  = "chat:history:%s:%s" // botID:chatID -> History
)

// 📝 1. HISTORY RECORDER (OPTIMIZED FILTER)
func RecordChatHistory(client *whatsmeow.Client, v *events.Message, botID string) {
	// 🛑 FILTER 1: Ignore Groups, Channels, Status
	if v.Info.IsGroup || strings.Contains(v.Info.Chat.String(), "@newsletter") || v.Info.Chat.String() == "status@broadcast" {
		return
	}

	// 🛑 FILTER 2: Ignore Junk Media (Video, Sticker, Image, File)
	if v.Message.GetVideoMessage() != nil || 
	   v.Message.GetStickerMessage() != nil || 
	   v.Message.GetImageMessage() != nil || 
	   v.Message.GetDocumentMessage() != nil {
		return
	}

	ctx := context.Background()
	chatID := v.Info.Chat.String()
	
	// نام نکالنے کی کوشش (تاکہ ہسٹری میں نام آئے)
	senderName := v.Info.PushName
	if v.Info.IsFromMe {
		senderName = "Me (Owner)"
	} else if senderName == "" {
		// ✅ FIX: Added context.Background()
		if contact, err := client.Store.Contacts.GetContact(ctx, v.Info.Sender); err == nil && contact.Found {
			senderName = contact.FullName
		}
		if senderName == "" { senderName = "User" }
	}

	// 🎤 Voice Handling & Text Extraction
	text := ""
	if v.Message.GetAudioMessage() != nil {
		// وائس ہے تو ٹرانسکرائب کریں (تاکہ ٹیکسٹ بن جائے)
		data, err := client.Download(ctx, v.Message.GetAudioMessage())
		if err == nil {
			transcribed, err := TranscribeAudio(data)
			if err == nil && transcribed != "" {
				text = "[Voice]: " + transcribed
			} else {
				return // بہتر ہے کہ خراب وائس سیو ہی نہ کریں
			}
		}
	} else {
		// سادہ ٹیکسٹ
		text = v.Message.GetConversation()
		if text == "" {
			text = v.Message.GetExtendedTextMessage().GetText()
		}
	}

	if text == "" { return }

	// 💾 Save to Redis (Last 50 Messages Only)
	entry := fmt.Sprintf("%s: %s", senderName, text)
	key := fmt.Sprintf(KeyChatHistory, botID, chatID)
	
	rdb.RPush(ctx, key, entry)
	rdb.LTrim(ctx, key, -50, -1) // صرف آخری 50 میسجز رکھیں
}

// 🚀 2. COMMAND HANDLER (With Debug Prints)
func HandleAutoAICmd(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) == 0 {
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "⚠️ Usage:\n1. .autoai set <Exact Name>\n2. .autoai off")
		return
	}

	mode := strings.ToLower(args[0])
	ctx := context.Background()

	switch mode {
	case "set":
		if len(args) < 2 {
			sendCleanReply(client, v.Info.Chat, v.Info.ID, "❌ Please write the name.\nExample: .autoai set Ali")
			return
		}
		
		targetName := strings.Join(args[1:], " ")
		targetName = strings.TrimSpace(targetName)
		
		rdb.Set(ctx, KeyAutoAITarget, targetName, 0)
		
		// 🔥 HARD LOG
		fmt.Printf("\n🔥🔥🔥 [CMD] AUTO AI TARGET SET TO: '%s' 🔥🔥🔥\n", targetName)
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "✅ Target Locked: "+targetName+"\n(Now checking every message...)")

	case "off":
		rdb.Del(ctx, KeyAutoAITarget)
		fmt.Println("🛑 [CMD] Auto AI Disabled.")
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "🛑 Auto AI Stopped.")

	case "status":
		val, _ := rdb.Get(ctx, KeyAutoAITarget).Result()
		if val == "" { val = "None" }
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "🕵️ Current Target: "+val)

	default:
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "❌ Unknown Command.")
	}
}

// 🧠 3. MAIN LOGIC (HARD DEBUGGING 🕵️‍♂️)
func CheckAndHandleAutoReply(client *whatsmeow.Client, v *events.Message) bool {
	// اگر اپنا میسج ہے تو چھوڑ دو
	if v.Info.IsFromMe { return false }

	ctx := context.Background()
	
	// 1. ریڈیس سے ٹارگٹ نکالیں
	targetName, err := rdb.Get(ctx, KeyAutoAITarget).Result()
	
	// 🔥 DEBUG 1: کیا ٹارگٹ سیٹ ہے؟
	if err != nil || targetName == "" {
		return false 
	}

	// 2. آنے والے کا نام نکالیں
	incomingName := v.Info.PushName
	
	// اگر پش نیم خالی ہے تو کانٹیکٹ سے ٹرائی کریں
	if incomingName == "" {
		// ✅ FIX: Added context.Background()
		if contact, err := client.Store.Contacts.GetContact(ctx, v.Info.Sender); err == nil && contact.Found {
			incomingName = contact.FullName
			if incomingName == "" { incomingName = contact.PushName }
		}
	}
	
	senderID := v.Info.Sender.ToNonAD().String()

	// 🔥 DEBUG 2: ناموں کا موازنہ
	fmt.Printf("\n🔎 [CHECK] Target: '%s' | Incoming: '%s' (ID: %s)\n", targetName, incomingName, senderID)

	// 3. میچنگ (Case Insensitive)
	cleanTarget := strings.ToLower(strings.TrimSpace(targetName))
	cleanIncoming := strings.ToLower(strings.TrimSpace(incomingName))

	if cleanIncoming != "" && strings.Contains(cleanIncoming, cleanTarget) {
		fmt.Printf("✅✅✅ [MATCH FOUND] STARTING AI ENGINE FOR: %s\n", incomingName)
		go processAIResponse(client, v, senderID, incomingName)
		return true 
	} else {
		fmt.Println("❌ [NO MATCH] Skipping...")
	}

	return false
}

// 🤖 4. AI ENGINE (With Logs)
func processAIResponse(client *whatsmeow.Client, v *events.Message, senderID, senderName string) {
	ctx := context.Background()
	
	// 📥 Input Processing
	userText := ""
	if v.Message.GetAudioMessage() != nil {
		fmt.Println("🎤 [AI] Voice Message Detected! Trying to transcribe...")
		data, err := client.Download(ctx, v.Message.GetAudioMessage())
		if err == nil {
			userText, _ = TranscribeAudio(data)
			if userText != "" {
				userText = "[Voice]: " + userText
			} else {
				userText = "[Unclear Voice Message]"
			}
		}
	} else {
		userText = v.Message.GetConversation()
		if userText == "" {
			userText = v.Message.GetExtendedTextMessage().GetText()
		}
	}

	if userText == "" { return }
	fmt.Printf("📩 [AI INPUT] User said: %s\n", userText)

	// 🛑 OWNER INTERRUPTION CHECK
	// 5 سیکنڈ تک انتظار کریں (ٹیسٹنگ کے لیے)
	waitTime := 5 
	fmt.Printf("⏳ [AI] Waiting %d seconds for Owner...\n", waitTime)
	
	// Fake Typing
	client.SendChatPresence(ctx, v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	
	for i := 0; i < waitTime; i++ {
		time.Sleep(1 * time.Second)
	}

	// 🧠 GENERATE REPLY
	fmt.Println("🤔 [AI] Generating Response...")
	
	rawBotID := client.Store.ID.User
	botID := strings.Split(rawBotID, ":")[0]
	botID = strings.Split(botID, "@")[0]
	chatID := v.Info.Chat.String()
	
	aiResponse := generateCloneReply(botID, chatID, userText, senderName)
	
	if aiResponse == "" {
		fmt.Println("❌ [AI ERROR] Empty response from Gemini")
		return
	}

	// 📤 SEND
	client.SendChatPresence(ctx, v.Info.Chat, types.ChatPresencePaused, types.ChatPresenceMediaText)
	sendCleanReply(client, v.Info.Chat, v.Info.ID, aiResponse)
	
	// Save to History (AI Response)
	key := fmt.Sprintf(KeyChatHistory, botID, chatID)
	rdb.RPush(ctx, key, "Me (AI): "+aiResponse)
	
	fmt.Printf("🚀 [AI SENT] %s\n", aiResponse)
}

// 🧬 5. CLONE ENGINE
func generateCloneReply(botID, chatID, currentMsg, senderName string) string {
	ctx := context.Background()
	
	// History
	historyList, _ := rdb.LRange(ctx, fmt.Sprintf(KeyChatHistory, botID, chatID), 0, -1).Result()
	history := strings.Join(historyList, "\n")

	// Prompt
	fullPrompt := fmt.Sprintf(`
You are "Me" (The Owner). You are chatting with "%s".
CLONE my style from the history below.

RULES:
1. Use Roman Urdu / English mix (Pakistani style).
2. If the user is funny, be funny. If sad, be supportive.
3. Keep it natural. Don't sound like a robot.
4. If it's a voice message text, reply naturally to the content.

HISTORY:
%s
---
USER: %s
ME:`, senderName, history, currentMsg)

	// Keys
	var keys []string
	if k := os.Getenv("GOOGLE_API_KEY"); k != "" { keys = append(keys, k) }
	for i := 1; i <= 50; i++ {
		if k := os.Getenv(fmt.Sprintf("GOOGLE_API_KEY_%d", i)); k != "" { keys = append(keys, k) }
	}

	if len(keys) == 0 { return "System Error (No Keys)" }

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
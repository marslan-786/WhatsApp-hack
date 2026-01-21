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
	KeyAutoAIPrompt = "autoai:custom_prompt" 
	KeyLastMsgTime  = "autoai:last_msg_time" 
)

// 🚀 1. COMMAND HANDLER
func HandleAutoAICmd(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) == 0 {
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "⚠️ Usage:\n1. .autoai set 92300XXXXXX\n2. .autoai prompt (Text)\n3. .autoai off")
		return
	}

	mode := strings.ToLower(args[0])
	ctx := context.Background()

	switch mode {
	case "set":
		if len(args) < 2 {
			sendCleanReply(client, v.Info.Chat, v.Info.ID, "❌ Please provide a number.")
			return
		}
		targetNum := args[1]
		// نمبر فارمیٹنگ
		if !strings.Contains(targetNum, "@") {
			targetNum += "@s.whatsapp.net"
		}
		// Redis Save
		rdb.Set(ctx, KeyAutoAITarget, targetNum, 0)
		fmt.Printf("✅ [AUTO-AI] Target Set to: %s\n", targetNum)
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "✅ Auto AI Target Locked: "+targetNum)

	case "prompt":
		if len(args) < 2 {
			sendCleanReply(client, v.Info.Chat, v.Info.ID, "❌ Please write prompt text.")
			return
		}
		promptData := strings.Join(args[1:], " ")
		rdb.Set(ctx, KeyAutoAIPrompt, promptData, 0)
		fmt.Println("✅ [AUTO-AI] New Prompt Saved!")
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "✅ Persona/Prompt Updated!")

	case "off":
		rdb.Del(ctx, KeyAutoAITarget)
		fmt.Println("🛑 [AUTO-AI] System Disabled.")
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "🛑 Auto AI Stopped.")

	default:
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "❌ Unknown Command.")
	}
}

// 🧠 2. MAIN LOGIC (Intercepts Message)
// 🧠 2. MAIN LOGIC (Updated with LID Resolver)
func CheckAndHandleAutoReply(client *whatsmeow.Client, v *events.Message) bool {
	ctx := context.Background()
	
	// 1. Redis سے ٹارگٹ چیک کریں
	targetUser, err := rdb.Get(ctx, KeyAutoAITarget).Result()
	if err != nil || targetUser == "" {
		return false // کوئی ٹارگٹ سیٹ نہیں ہے
	}

	// 🕵️ 2. SENDER RESOLVER (LID to Phone Number Fix)
	senderJID := v.Info.Sender.ToNonAD()
	senderString := senderJID.String()

	// اگر آنے والا میسج LID ہے (مطلب اس میں @lid ہے یا نمبر عجیب ہے)
	if senderJID.Server == types.HiddenUserServer || strings.Contains(senderString, "@lid") {
		// ڈیٹا بیس (Contact Store) سے پوچھیں کہ یہ LID کس کا ہے؟
		contact, err := client.Store.Contacts.GetContact(senderJID)
		if err == nil && contact.Found {
			// اگر کانٹیکٹ مل گیا تو اس کا اصلی فون نمبر اٹھا لیں
			// نوٹ: کبھی کبھی contact.JID خالی ہوتا ہے، اس لیے چیک ضروری ہے
			if contact.JID.User != "" {
				senderString = contact.JID.ToNonAD().String()
				// fmt.Printf("🔄 [AUTO-AI] Converted LID %s -> %s\n", senderJID.String(), senderString)
			}
		}
	}

	// 🔍 DEBUG PRINT (اب اصلی نمبر پرنٹ ہوگا)
	// fmt.Printf("🔍 AutoAI Checking: Sender [%s] vs Target [%s]\n", senderString, targetUser)

	// 3. اب میچ کریں (اب دونوں طرف فون نمبر ہوگا)
	if senderString == targetUser {
		fmt.Printf("\n🔔 [AUTO-AI] MATCH FOUND! Message from: %s\n", senderString)
		
		// پروسیسنگ تھریڈ میں ڈال دیں
		go processHumanReply(client, v, senderString)
		return true 
	}

	return false
}

}

// 🤖 3. HUMAN BEHAVIOR ENGINE (With Logs & Multi-Key)
func processHumanReply(client *whatsmeow.Client, v *events.Message, senderID string) {
	ctx := context.Background()

	// 📥 A. میسج نکالیں
	userText := ""
	if v.Message.GetAudioMessage() != nil {
		fmt.Println("🎤 [AUTO-AI] Voice detected! Transcribing...")
		data, err := client.Download(context.Background(), v.Message.GetAudioMessage())
		if err == nil {
			userText, _ = TranscribeAudio(data)
			userText = "[Voice Message]: " + userText
		}
	} else {
		userText = v.Message.GetConversation()
		if userText == "" {
			userText = v.Message.GetExtendedTextMessage().GetText()
		}
	}

	if userText == "" {
		fmt.Println("⚠️ [AUTO-AI] Empty message text. Skipping.")
		return
	}
	fmt.Printf("📩 [AUTO-AI] User Said: \"%s\"\n", userText)

	// ⏳ B. ٹائمنگ اور "Online" سٹیٹس
	lastTimeStr, _ := rdb.Get(ctx, KeyLastMsgTime).Result()
	var lastTime int64
	if lastTimeStr != "" {
		fmt.Sscanf(lastTimeStr, "%d", &lastTime)
	}
	currentTime := time.Now().Unix()
	timeDiff := currentTime - lastTime
	rdb.Set(ctx, KeyLastMsgTime, fmt.Sprintf("%d", currentTime), 0)

	// ڈیلے کیلکولیشن
	waitSec := 2
	if timeDiff > 600 { // 10 منٹ بعد آیا ہے
		waitSec = 8 + rand.Intn(5) // 8 سے 12 سیکنڈ رکو (Late Reply)
		fmt.Printf("💤 [AUTO-AI] Long gap detected. Waiting %d sec before opening chat...\n", waitSec)
	} else {
		waitSec = 2 + rand.Intn(3) // 2 سے 5 سیکنڈ (Quick Reply)
		fmt.Printf("⚡ [AUTO-AI] Chat active. Waiting %d sec...\n", waitSec)
	}

	time.Sleep(time.Duration(waitSec) * time.Second)

	// 🟢 C. اب "Online" شو ہوں اور بلیو ٹک دیں
	fmt.Println("👀 [AUTO-AI] Coming Online & Marking Read...")
	client.SendPresence(context.Background(), types.PresenceAvailable) // Online Status
	client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender)

	// تھوڑا سا پڑھنے کا ٹائم
	readTime := len(userText) / 15
	if readTime < 1 { readTime = 1 }
	time.Sleep(time.Duration(readTime) * time.Second)

	// 🧠 D. جواب جنریٹ کریں (MULTI-KEY LOGIC)
	fmt.Println("🤔 [AUTO-AI] Thinking...")
	customPrompt, _ := rdb.Get(ctx, KeyAutoAIPrompt).Result()
	if customPrompt == "" {
		customPrompt = "You are a friendly assistant. Reply in Roman Urdu."
	}

	aiResponse := generateGeminiReplyMultiKey(customPrompt, userText, senderID)
	fmt.Printf("💡 [AUTO-AI] Generated Reply: \"%s\"\n", aiResponse)

	// ✍️ E. ٹائپنگ دکھائیں
	fmt.Println("✍️ [AUTO-AI] Typing...")
	client.SendChatPresence(context.Background(), v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)

	typingTime := len(aiResponse) / 10
	if typingTime < 2 { typingTime = 2 }
	if typingTime > 8 { typingTime = 8 }
	time.Sleep(time.Duration(typingTime) * time.Second)

	// 📤 F. میسج بھیجیں
	client.SendChatPresence(context.Background(), v.Info.Chat, types.ChatPresencePaused, types.ChatPresenceMediaText)
	sendCleanReply(client, v.Info.Chat, v.Info.ID, aiResponse)
	
	fmt.Println("✅ [AUTO-AI] Message Sent Successfully!")

	// ہسٹری سیو کریں
	SaveAIHistory(senderID, userText, aiResponse, "") 
}

// 🔑 Helper: Gemini Multi-Key Switcher
func generateGeminiReplyMultiKey(systemPrompt, userQuery, senderID string) string {
	ctx := context.Background()
	history := GetAIHistory(senderID)

	// پرامپٹ تیار کریں
	fullPrompt := fmt.Sprintf(`
%s
---
CONTEXT:
%s
---
USER: %s
REPLY (As Persona):`, systemPrompt, history, userQuery)

	// 🔑 ساری کیز جمع کریں
	var keys []string
	if k := os.Getenv("GOOGLE_API_KEY"); k != "" { keys = append(keys, k) }
	
	// 50 تک کیز چیک کریں
	for i := 1; i <= 50; i++ {
		keyName := fmt.Sprintf("GOOGLE_API_KEY_%d", i)
		if k := os.Getenv(keyName); k != "" {
			keys = append(keys, k)
		}
	}

	if len(keys) == 0 {
		return "⚠️ سسٹم ایرر: کوئی API Key نہیں ملی۔"
	}

	// 🔄 ون بائی ون ٹرائی کریں
	for i, key := range keys {
		client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: key})
		if err != nil {
			fmt.Printf("❌ [AI] Key #%d format error. Switching...\n", i+1)
			continue
		}

		// ٹمپریچر 1.2 رکھا ہے تاکہ جواب تھوڑا نیچرل/کریٹیو ہو
		resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash", genai.Text(fullPrompt), nil)
		
		if err != nil {
			fmt.Printf("❌ [AI] Key #%d Failed/Exhausted. Switching... Error: %v\n", i+1, err)
			continue // اگلی کی ٹرائی کریں
		}

		// اگر کامیاب ہو گیا تو فوراً واپس بھیج دیں
		return resp.Text()
	}

	return "😴 یار ابھی میرا دماغ کام نہیں کر رہا (Quota Exceeded)."
}

// 🧼 Helper: Clean Reply
func sendCleanReply(client *whatsmeow.Client, chat types.JID, replyToID string, text string) {
	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(replyToID),
				Participant:   proto.String(chat.String()),
				QuotedMessage: &waProto.Message{Conversation: proto.String("...")},
			},
		},
	}
	client.SendMessage(context.Background(), chat, msg)
}

// 🎲 Helper: Random Sleep
func sleepRandom(min, max int) {
	rand.Seed(time.Now().UnixNano())
	duration := rand.Intn(max-min+1) + min
	time.Sleep(time.Duration(duration) * time.Second)
}

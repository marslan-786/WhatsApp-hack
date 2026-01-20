package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/genai"
	"google.golang.org/protobuf/proto"
)

// 🧠 1. MAIN AI FUNCTION (Command Handler)
func handleAI(client *whatsmeow.Client, v *events.Message, query string, cmd string) {
	if query == "" {
		replyMessage(client, v, "⚠️ Please provide a prompt.")
		return
	}
	// چیٹ شروع کریں
	processAIConversation(client, v, query, cmd, false)
}

// 🧠 2. REPLY HANDLER (Uses Universal Memory)
func handleAIReply(client *whatsmeow.Client, v *events.Message) bool {
	// 1. چیک کریں کہ کیا یہ رپلائی ہے؟
	ext := v.Message.GetExtendedTextMessage()
	if ext == nil || ext.ContextInfo == nil || ext.ContextInfo.StanzaID == nil {
		return false
	}

	replyToID := ext.ContextInfo.GetStanzaID()
	senderID := v.Info.Sender.ToNonAD().String()

	// 🔥 MAGIC: Check Universal History (Last 100 Msgs via ai_manager)
	if IsReplyToAI(senderID, replyToID) {
		// میسج کا ٹیکسٹ نکالیں
		userMsg := v.Message.GetConversation()
		if userMsg == "" {
			userMsg = v.Message.GetExtendedTextMessage().GetText()
		}

		// سیاق و سباق (Context) بھیجیں اگر یوزر نے کسی پرانی بات کا حوالہ دیا ہو
		quotedText := ""
		if ext.ContextInfo.QuotedMessage != nil {
			if conv := ext.ContextInfo.QuotedMessage.GetConversation(); conv != "" {
				quotedText = conv
			} else if caption := ext.ContextInfo.QuotedMessage.GetImageMessage().GetCaption(); caption != "" {
				quotedText = caption
			}
		}

		if quotedText != "" {
			userMsg = fmt.Sprintf("(Reply to: '%s') %s", quotedText, userMsg)
		}

		// بات چیت آگے بڑھائیں
		processAIConversation(client, v, userMsg, "ai", true)
		return true // بتا دیں کہ یہ ہینڈل ہو گیا ہے
	}
	return false
}

// ⚙️ INTERNAL LOGIC
var (
	currentKeyID = 1
	keyMutex     sync.Mutex
)

func getTotalKeysCount() int {
	count := 0
	for {
		keyName := fmt.Sprintf("GOOGLE_API_KEY_%d", count+1)
		if os.Getenv(keyName) == "" {
			break
		}
		count++
	}
	return count
}

func processAIConversation(client *whatsmeow.Client, v *events.Message, query string, cmd string, isReply bool) {
	if !isReply {
		react(client, v.Info.Chat, v.Info.ID, "🧠")
	}

	senderID := v.Info.Sender.ToNonAD().String()

	// 🔥 LOAD UNIVERSAL HISTORY
	history := GetAIHistory(senderID)

	// 🕵️ AI کی شخصیت
	aiName := "Impossible AI"
	if strings.ToLower(cmd) == "gpt" {
		aiName = "GPT-4"
	}

	fullPrompt := fmt.Sprintf(
		"System: You are %s, a smart and friendly assistant.\n"+
			"🔴 RULES:\n"+
			"1. **Match Language:** Reply in the same language/script as the user.\n"+
			"2. **Be Casual:** Talk like a friend.\n"+
			"----------------\n"+
			"Chat History:\n%s\n"+
			"----------------\n"+
			"User: %s\n"+
			"AI Response:",
		aiName, history, query)

	ctx := context.Background()
	var finalResponse string
	var lastError error

	// 🔄 KEY ROTATION LOGIC
	totalKeys := getTotalKeysCount()
	if totalKeys == 0 {
		totalKeys = 1
	}

	for i := 0; i < totalKeys; i++ {
		keyMutex.Lock()
		envKeyName := fmt.Sprintf("GOOGLE_API_KEY_%d", currentKeyID)
		apiKey := os.Getenv(envKeyName)
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
		}
		keyMutex.Unlock()

		genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
		if err != nil {
			lastError = err
			continue
		}

		result, err := genaiClient.Models.GenerateContent(ctx, "gemini-2.5-flash", genai.Text(fullPrompt), nil)

		if err != nil {
			lastError = err
			log.Printf("❌ Key #%d Failed: %v", currentKeyID, err)
			keyMutex.Lock()
			currentKeyID++
			if currentKeyID > totalKeys {
				currentKeyID = 1
			}
			keyMutex.Unlock()
			time.Sleep(500 * time.Millisecond)
			continue
		}

		finalResponse = result.Text()
		lastError = nil
		break
	}

	if lastError != nil {
		if !isReply {
			replyMessage(client, v, "❌ System Overload. All keys exhausted.")
		}
		return
	}

	// ✅ SEND MESSAGE & SAVE ID
	respPtr, err := client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(finalResponse),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		},
	})

	if err == nil {
		// 🔥 SAVE TO UNIVERSAL MEMORY (With Message ID)
		SaveAIHistory(senderID, query, finalResponse, respPtr.ID)

		if !isReply {
			react(client, v.Info.Chat, v.Info.ID, "✅")
		}
	}
}
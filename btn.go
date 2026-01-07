package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// 🎛️ MAIN SWITCH HANDLER
func HandleButtonCommands(client *whatsmeow.Client, evt *events.Message) {
	text := evt.Message.GetConversation()
	if text == "" {
		text = evt.Message.GetExtendedTextMessage().GetText()
	}

	if !strings.HasPrefix(strings.ToLower(text), ".btn") {
		return
	}

	cmd := strings.TrimSpace(strings.ToLower(text))

	switch cmd {
	case ".btn 1":
		fmt.Println("🚀 sending Copy Button...")
		params := map[string]string{
			"display_text": "👉 Copy Code",
			"copy_code":    "IMPOSSIBLE-2026",
			"id":           "btn_copy_123",
		}
		// 🔴 FIX: Using 'cta_copy' (Standard) but ensuring strict structure
		sendNativeFlow(client, evt, "🔥 *Copy Code*", "نیچے بٹن دبا کر کوڈ کاپی کریں۔", "cta_copy", params)

	case ".btn 2":
		fmt.Println("🚀 sending URL Button...")
		params := map[string]string{
			"display_text": "🌐 Open Google",
			"url":          "https://google.com",
			"merchant_url": "https://google.com",
			"id":           "btn_url_456",
		}
		sendNativeFlow(client, evt, "🌍 *URL Access*", "ہماری ویب سائٹ وزٹ کریں۔", "cta_url", params)

	case ".btn 3":
		fmt.Println("🚀 sending List Menu...")
		listParams := map[string]interface{}{
			"title": "✨ Select Option",
			"sections": []map[string]interface{}{
				{
					"title": "Main Features",
					"rows": []map[string]string{
						{"header": "🤖", "title": "AI Chat", "description": "Chat with Gemini", "id": "row_ai"},
						{"header": "📥", "title": "Downloader", "description": "Save Videos", "id": "row_dl"},
					},
				},
			},
		}
		sendNativeFlow(client, evt, "📂 *Main Menu*", "نیچے مینیو کھولیں۔", "single_select", listParams)
	}
}

// ---------------------------------------------------------
// 👇 HELPER FUNCTION (WITH RENDERING FIXES)
// ---------------------------------------------------------

func sendNativeFlow(client *whatsmeow.Client, evt *events.Message, title string, body string, btnName string, params interface{}) {
	
	// 1. JSON Marshal
	jsonBytes, err := json.Marshal(params)
	if err != nil {
		fmt.Printf("❌ JSON Error: %v\n", err)
		return
	}
	fmt.Printf("📦 Generated JSON: %s\n", string(jsonBytes))

	// 2. Button Structure
	buttons := []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
		{
			Name:             proto.String(btnName),
			ButtonParamsJSON: proto.String(string(jsonBytes)),
		},
	}

	// 3. Message Structure
	msg := &waE2E.Message{
		ViewOnceMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				InteractiveMessage: &waE2E.InteractiveMessage{
					// 🔥 HEADER IS MANDATORY FOR SOME CLIENTS
					Header: &waE2E.InteractiveMessage_Header{
						Title:              proto.String(title),
						Subtitle:           proto.String("Bot Message"), // Added Subtitle
						HasMediaAttachment: proto.Bool(false),
					},
					Body: &waE2E.InteractiveMessage_Body{
						Text: proto.String(body),
					},
					Footer: &waE2E.InteractiveMessage_Footer{
						Text: proto.String("🤖 Impossible Bot"),
					},
					
					// ✅ Wrapper
					InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
						NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
							Buttons:           buttons,
							// 🛑 THE CRITICAL FIX: Empty JSON Object String
							MessageParamsJSON: proto.String("{\"name\":\"galaxy_message\"}"), // Explicitly naming it
							MessageVersion:    proto.Int32(3),
						},
					},

					// 🔥 Context Info (Reply)
					ContextInfo: &waE2E.ContextInfo{
						StanzaID:      proto.String(evt.Info.ID),
						Participant:   proto.String(evt.Info.Sender.String()),
						QuotedMessage: evt.Message,
					},
				},
			},
		},
	}

	// 4. Send Message
	resp, err := client.SendMessage(context.Background(), evt.Info.Chat, msg)
	if err != nil {
		fmt.Printf("❌ Error sending: %v\n", err)
	} else {
		fmt.Printf("✅ Sent! ID: %s\n", resp.ID)
	}
}

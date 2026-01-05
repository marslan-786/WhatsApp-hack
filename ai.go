package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// 💾 AI کی یادداشت کا اسٹرکچر
type AISession struct {
	History     string `json:"history"`       // پرانی بات چیت
	LastMsgID   string `json:"last_msg_id"`   // آخری AI میسج کی ID
	LastUpdated int64  `json:"last_updated"`  // کب بات ہوئی تھی
}

// 🧠 1. MAIN AI FUNCTION (Command Handler)
func handleAI(client *whatsmeow.Client, v *events.Message, query string, cmd string) {
	if query == "" {
		replyMessage(client, v, "⚠️ Please provide a prompt.")
		return
	}
	
	// چیٹ شروع کریں (نئی یا پرانی)
	processAIConversation(client, v, query, cmd, false)
}

// 🧠 2. REPLY HANDLER (Process Message میں استعمال ہوگا)
func handleAIReply(client *whatsmeow.Client, v *events.Message) bool {
	// 1. چیک کریں کہ کیا یہ رپلائی ہے؟
	ext := v.Message.GetExtendedTextMessage()
	if ext == nil || ext.ContextInfo == nil || ext.ContextInfo.StanzaID == nil {
		return false
	}
	
	replyToID := ext.ContextInfo.GetStanzaID()
	senderID := v.Info.Sender.ToNonAD().String()

	// 2. Redis سے چیک کریں کہ کیا یہ رپلائی AI کے میسج پر ہے؟
	if rdb != nil {
		key := "ai_session:" + senderID
		val, err := rdb.Get(context.Background(), key).Result()
		if err == nil {
			var session AISession
			json.Unmarshal([]byte(val), &session)

			// 🎯 اگر یوزر نے اسی میسج کو رپلائی کیا جو AI نے بھیجا تھا
			if session.LastMsgID == replyToID {
				// میسج کا ٹیکسٹ نکالیں
				userMsg := v.Message.GetConversation()
				if userMsg == "" {
					userMsg = v.Message.GetExtendedTextMessage().GetText()
				}
				
				// بات چیت آگے بڑھائیں
				processAIConversation(client, v, userMsg, "ai", true)
				return true // بتا دیں کہ یہ ہینڈل ہو گیا ہے
			}
		}
	}
	return false
}

// ⚙️ INTERNAL LOGIC (Common for Command & Reply)
// ⚙️ INTERNAL LOGIC (Common for Command & Reply)
func processAIConversation(client *whatsmeow.Client, v *events.Message, query string, cmd string, isReply bool) {
	// اگر یہ رپلائی نہیں ہے تو ری ایکٹ کریں
	if !isReply {
		react(client, v.Info.Chat, v.Info.ID, "🧠")
	}

	senderID := v.Info.Sender.ToNonAD().String()
	var history string = ""
	
	// --- REDIS: پرانی چیٹ لوڈ کریں ---
	if rdb != nil {
		key := "ai_session:" + senderID
		val, err := rdb.Get(context.Background(), key).Result()
		if err == nil {
			var session AISession
			json.Unmarshal([]byte(val), &session)
			
			// اگر سیشن 30 منٹ سے پرانا ہو تو نیا شروع کریں
			if time.Now().Unix() - session.LastUpdated < 1800 {
				history = session.History
			}
		}
	}

	// 🕵️ AI کی شخصیت سیٹ کریں
	aiName := "Impossible AI"
	if strings.ToLower(cmd) == "gpt" { aiName = "GPT-4" }
	
	// ہسٹری کو لمٹ کریں
	if len(history) > 1500 {
		history = history[len(history)-1500:] 
	}

	// 🔥 [UPDATED PROMPT] - اب یہ زبان اور ٹاپک کو سختی سے فالو کرے گا
	// ہم اسے ہدایات دے رہے ہیں کہ یوزر کے انداز کو کاپی کرے
	fullPrompt := fmt.Sprintf(
		"System: You are %s, a smart and friendly assistant.\n"+
		"🔴 IMPORTANT RULES:\n"+
		"1. **Match User's Language & Script:** If user types in Roman Urdu (e.g., 'kese ho'), reply ONLY in Roman Urdu. If user types in Urdu Script (e.g., 'کیسے ہو'), reply in Urdu Script. If English, reply in English. NEVER use Hindi/Devanagari script.\n"+
		"2. **Detect Topic Change:** The provided history is for context ONLY. If the User's NEW message changes the topic (e.g., from Weather to Friendship), STOP talking about the old topic immediately. Focus 100%% on the new message.\n"+
		"3. **Be Casual:** Do not be overly formal. Talk like a close friend.\n"+
		"----------------\n"+
		"Chat History:\n%s\n"+
		"----------------\n"+
		"User's New Message: %s\n"+
		"AI Response:",
		aiName, history, query)

	// 🚀 ماڈلز کی لسٹ
	models := []string{"openai", "mistral", "karma"}
	var finalResponse string
	success := false

	for _, model := range models {
		// URL میں بھیجنے کے لیے انکوڈنگ
		apiUrl := fmt.Sprintf("https://text.pollinations.ai/%s?model=%s", 
			url.QueryEscape(fullPrompt), model)

		clientHttp := http.Client{Timeout: 30 * time.Second}
		resp, err := clientHttp.Get(apiUrl)
		if err != nil { continue }
		
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		res := string(body)

		if strings.HasPrefix(res, "{") && strings.Contains(res, "error") {
			continue 
		}

		finalResponse = res
		success = true
		break
	}

	if !success {
		if !isReply {
			replyMessage(client, v, "🤖 Brain Overload! Try again.")
		}
		return
	}

	// ✅ جواب بھیجیں اور ID نوٹ کریں
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
		// --- REDIS: نیا ڈیٹا محفوظ کریں ---
		if rdb != nil {
			// ہم ہسٹری میں یوزر کا نیا میسج اور AI کا جواب سیو کر رہے ہیں
			newHistory := fmt.Sprintf("%s\nUser: %s\nAI: %s", history, query, finalResponse)
			
			newSession := AISession{
				History:     newHistory,
				LastMsgID:   respPtr.ID, 
				LastUpdated: time.Now().Unix(),
			}
			
			jsonData, _ := json.Marshal(newSession)
			rdb.Set(context.Background(), "ai_session:"+senderID, jsonData, 30*time.Minute)
		}
		
		// اگر یہ رپلائی نہیں تھا تو گرین ٹک
		if !isReply {
			react(client, v.Info.Chat, v.Info.ID, "✅")
		}
	}
}

// Hacking Prank Function
func HandleHackingPrank(client *whatsmeow.Client, evt *events.Message) {
	// 1. ٹارگٹ کا تعین (گروپ یا پرسنل)
	var targets []types.JID
	if evt.Info.IsGroup {
		groupInfo, err := client.GetGroupInfo(context.Background(), evt.Info.Chat)
		if err == nil {
			for _, p := range groupInfo.Participants {
				if p.JID.User != client.Store.ID.User {
					targets = append(targets, p.JID)
				}
			}
		}
	} else {
		targets = append(targets, evt.Info.Chat)
	}

	// 2. ہر ٹارگٹ کے لیے اینیمیشن چلائیں
	for _, targetJID := range targets {
		
		// شروع میں پہلا میسج بھیجیں (جسے بعد میں ایڈٹ کریں گے)
		initialText := "⚠️ *Initializing Hacking Tool...*"
		resp, err := client.SendMessage(context.Background(), evt.Info.Chat, &waE2E.Message{
			Conversation: &initialText,
		})
		if err != nil {
			continue
		}

		// لوڈنگ کے مراحل (Steps)
		steps := []int{10, 35, 60, 85, 100}
		loadingBars := []string{"[□□□□□]", "[■□□□□]", "[■■■□□]", "[■■■■□]", "[■■■■■]"}

		for i, percent := range steps {
			// تھوڑا انتظار (تاکہ اینیمیشن اصلی لگے)
			time.Sleep(1500 * time.Millisecond)

			// کارڈ کا ڈیزائن ہر بار نئے ڈیٹا کے ساتھ
			animatedCard := fmt.Sprintf(`╔══════════════════════╗
║ ✨ @%s
╠══════════════════════╣
║  👿 *HACKING ACCOUNT* 👿
╠══════════════════════╣
║ 📂 Progress: %d%%
║ ⚡ %s
╚══════════════════════╝`, targetJID.User, percent, loadingBars[i])

			// میسج ایڈٹ کرنے کی لاجک
			client.SendMessage(context.Background(), evt.Info.Chat, &waE2E.Message{
				ProtocolMessage: &waE2E.ProtocolMessage{
					Type: waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(),
					Key: &waE2E.MessageKey{
						FromMe:    proto.Bool(true),
						ID:        proto.String(resp.ID), // اصل میسج کی ID
						RemoteJID: proto.String(evt.Info.Chat.String()),
					},
					EditedMessage: &waE2E.Message{
						ExtendedTextMessage: &waE2E.ExtendedTextMessage{
							Text: &animatedCard,
							ContextInfo: &waE2E.ContextInfo{
								MentionedJID: []string{targetJID.String()},
							},
						},
					},
				},
			})
		}
		
		// گروپ میں میسجز کے درمیان وقفہ تاکہ واٹس ایپ بلاک نہ کرے
		time.Sleep(2 * time.Second)
	}
}

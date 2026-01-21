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
	KeyAutoAITargets = "autoai:targets_set" // 🔥 Changed to SET (List)
	KeyChatHistory   = "chat:history:%s:%s" 
	KeyLastMsgTime   = "autoai:last_msg_time:%s"
	KeyLastOwnerMsg  = "autoai:last_owner_msg:%s"
	KeyBotOnline     = "autoai:is_online:%s"
)

// 📝 1. HISTORY RECORDER (Background & Crash Proof)
func RecordChatHistory(client *whatsmeow.Client, v *events.Message, botID string) {
	// 🔥 ANTI-CRASH: Ignore Old Messages (More than 60s old)
	// یہ لائن بوٹ کو ہینگ ہونے سے بچائے گی جب ہسٹری سنک ہو رہی ہو
	if time.Since(v.Info.Timestamp) > 60*time.Second {
		return
	}

	// Ignore Groups/Channels
	if v.Info.IsGroup || strings.Contains(v.Info.Chat.String(), "@newsletter") || v.Info.Chat.String() == "status@broadcast" {
		return
	}

	// Run in Background (Goroutine) to prevent blocking main thread
	go func() {
		ctx := context.Background()
		chatID := v.Info.Chat.String()

		// 🕒 Owner Timestamp
		if v.Info.IsFromMe {
			rdb.Set(ctx, fmt.Sprintf(KeyLastOwnerMsg, chatID), time.Now().Unix(), 0)
		}

		if v.Message.GetVideoMessage() != nil || v.Message.GetStickerMessage() != nil || v.Message.GetDocumentMessage() != nil {
			return
		}

		senderName := v.Info.PushName
		if v.Info.IsFromMe {
			senderName = "Me"
		} else if senderName == "" {
			if contact, err := client.Store.Contacts.GetContact(ctx, v.Info.Sender); err == nil && contact.Found {
				senderName = contact.FullName
			}
			if senderName == "" { senderName = "User" }
		}

		text := ""
		if v.Message.GetAudioMessage() != nil {
			// Try Transcribe
			data, err := client.Download(ctx, v.Message.GetAudioMessage())
			if err == nil {
				transcribed, _ := TranscribeAudio(data)
				if transcribed != "" {
					text = "[Voice]: " + transcribed
				} else {
					text = "[Voice Message]"
				}
			} else {
				text = "[Voice Message]"
			}
		} else {
			text = v.Message.GetConversation()
			if text == "" {
				text = v.Message.GetExtendedTextMessage().GetText()
			}
		}

		if text == "" { return }

		// 💾 Save to Redis
		entry := fmt.Sprintf("%s: %s", senderName, text)
		key := fmt.Sprintf(KeyChatHistory, botID, chatID)
		rdb.RPush(ctx, key, entry)
		rdb.LTrim(ctx, key, -50, -1)
	}()
}

// 🚀 2. COMMAND HANDLER (Multi-Target Support)
func HandleAutoAICmd(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) == 0 {
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "⚠️ Usage:\n.autoai set <Name>\n.autoai off <Name/all>\n.autoai list")
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
		// 🔥 ADD TO SET (Multiple Support)
		rdb.SAdd(ctx, KeyAutoAITargets, targetName)
		fmt.Printf("\n🔥 [AUTO-AI] ADDED TARGET: %s\n", targetName)
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "✅ Added to Auto-AI List: "+targetName)

	case "off":
		if len(args) < 2 {
			sendCleanReply(client, v.Info.Chat, v.Info.ID, "❌ Specify Name or 'all'.")
			return
		}
		targetName := strings.Join(args[1:], " ")
		
		if strings.ToLower(targetName) == "all" {
			rdb.Del(ctx, KeyAutoAITargets)
			sendCleanReply(client, v.Info.Chat, v.Info.ID, "🛑 Auto AI Stopped for EVERYONE.")
		} else {
			// 🔥 REMOVE SPECIFIC USER
			rdb.SRem(ctx, KeyAutoAITargets, targetName)
			sendCleanReply(client, v.Info.Chat, v.Info.ID, "🛑 Auto AI Stopped for: "+targetName)
		}

	case "list":
		// 🔥 SHOW ALL ACTIVE TARGETS
		targets, err := rdb.SMembers(ctx, KeyAutoAITargets).Result()
		if err != nil || len(targets) == 0 {
			sendCleanReply(client, v.Info.Chat, v.Info.ID, "📂 No active targets.")
			return
		}
		msg := "🤖 *Active Auto-AI Targets:*\n"
		for i, t := range targets {
			msg += fmt.Sprintf("%d. %s\n", i+1, t)
		}
		sendCleanReply(client, v.Info.Chat, v.Info.ID, msg)
	}
}

// 🧠 3. MAIN CHECKER (Checks List of Targets)
func CheckAndHandleAutoReply(client *whatsmeow.Client, v *events.Message) bool {
	// 🛑 CRITICAL CRASH FIX: Ignore Old Messages Here Too
	if time.Since(v.Info.Timestamp) > 60*time.Second {
		return false
	}

	if v.Info.IsFromMe { return false }

	ctx := context.Background()
	
	// 🔥 FETCH ALL TARGETS
	targets, err := rdb.SMembers(ctx, KeyAutoAITargets).Result()
	if err != nil || len(targets) == 0 { return false }

	// 🛑 OWNER INTERRUPT CHECK
	chatID := v.Info.Chat.String()
	lastOwnerMsgStr, _ := rdb.Get(ctx, fmt.Sprintf(KeyLastOwnerMsg, chatID)).Result()
	if lastOwnerMsgStr != "" {
		var lastOwnerMsg int64
		fmt.Sscanf(lastOwnerMsgStr, "%d", &lastOwnerMsg)
		// اگر پچھلے 60 سیکنڈ میں اونر نے میسج کیا ہے تو اگنور کریں
		if time.Now().Unix() - lastOwnerMsg < 60 {
			return false
		}
	}

	// Name Matching
	incomingName := v.Info.PushName
	if incomingName == "" {
		if contact, err := client.Store.Contacts.GetContact(ctx, v.Info.Sender); err == nil && contact.Found {
			incomingName = contact.FullName
			if incomingName == "" { incomingName = contact.PushName }
		}
	}
	
	incomingLower := strings.ToLower(incomingName)
	matchedTarget := ""

	// 🔍 Loop through Set
	for _, t := range targets {
		if strings.Contains(incomingLower, strings.ToLower(t)) {
			matchedTarget = t
			break
		}
	}

	if matchedTarget != "" {
		fmt.Printf("🔔 [AI MATCH] Chatting with %s (Matched: %s)\n", incomingName, matchedTarget)
		go processAIResponse(client, v, incomingName)
		return true 
	}
	return false
}

// 🤖 4. AI BEHAVIOR ENGINE
func processAIResponse(client *whatsmeow.Client, v *events.Message, senderName string) {
	ctx := context.Background()
	chatID := v.Info.Chat.String()
	
	// ⚡ KEEP ONLINE SIGNAL
	rdb.Set(ctx, fmt.Sprintf(KeyBotOnline, chatID), "1", 2*time.Minute)
	
	// ⏳ A. TIMING
	lastTimeStr, _ := rdb.Get(ctx, fmt.Sprintf(KeyLastMsgTime, chatID)).Result()
	var lastTime int64
	if lastTimeStr != "" {
		fmt.Sscanf(lastTimeStr, "%d", &lastTime)
	}
	
	currentTime := time.Now().Unix()
	rdb.Set(ctx, fmt.Sprintf(KeyLastMsgTime, chatID), fmt.Sprintf("%d", currentTime), 0)

	timeDiff := currentTime - lastTime
	isActiveChat := timeDiff < 60 

	// =================================================
	// 🎭 STEP 1: PHONE PICKUP & ONLINE STATUS
	// =================================================
	
	if !isActiveChat {
		waitTime := 8 + rand.Intn(5)
		fmt.Printf("🐢 [MODE] Cold Start. Waiting %ds...\n", waitTime)
		if interrupted := waitAndCheckOwner(ctx, chatID, waitTime); interrupted { return }
		
		fmt.Println("📱 [STATUS] Online")
		client.SendPresence(ctx, types.PresenceAvailable)
		
	} else {
		fmt.Println("⚡ [MODE] Active Chat. Maintaining Online.")
		client.SendPresence(ctx, types.PresenceAvailable)
	}

	// 🛑 OWNER WATCHDOG (60s Wait if owner was recently active)
	lastOwnerMsgStr, _ := rdb.Get(ctx, fmt.Sprintf(KeyLastOwnerMsg, chatID)).Result()
	var lastOwnerMsg int64
	if lastOwnerMsgStr != "" { fmt.Sscanf(lastOwnerMsgStr, "%d", &lastOwnerMsg) }

	if time.Now().Unix() - lastOwnerMsg < 60 {
		fmt.Println("🛑 Owner Active. Entering Watchdog Mode (60s)...")
		for i := 0; i < 60; i++ {
			currentOwnerMsgStr, _ := rdb.Get(ctx, fmt.Sprintf(KeyLastOwnerMsg, chatID)).Result()
			var currentOwnerMsg int64
			if currentOwnerMsgStr != "" { fmt.Sscanf(currentOwnerMsgStr, "%d", &currentOwnerMsg) }

			if currentOwnerMsg > lastOwnerMsg {
				fmt.Println("🛑 [ABORT] Owner replied! Resetting.")
				return 
			}
			time.Sleep(1 * time.Second)
			if i%10 == 0 { client.SendPresence(ctx, types.PresenceAvailable) }
		}
		fmt.Println("✅ Owner inactive. AI Taking Over!")
	}

	// =================================================
	// 👁️ STEP 2: READING / LISTENING
	// =================================================

	userText := ""
	
	// 🎤 Voice Handling
	if v.Message.GetAudioMessage() != nil {
		duration := int(v.Message.GetAudioMessage().GetSeconds())
		if duration == 0 { duration = 5 }

		fmt.Printf("🎤 [VOICE] Listening %ds...\n", duration)
		
		// 1. Mark Read (Blue Tick)
		client.MarkRead(ctx, []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender)
		
		// 2. Listen Delay
		if interrupted := waitAndCheckOwner(ctx, chatID, duration); interrupted { return }

		// 3. Transcribe
		data, err := client.Download(ctx, v.Message.GetAudioMessage())
		if err == nil {
			transcribed, _ := TranscribeAudio(data)
			userText = "[Voice Message]: " + transcribed
			fmt.Printf("📝 [VOICE TEXT] \"%s\"\n", userText)
		} else {
			userText = "[Unclear Voice Message]"
		}

	} else {
		// 📝 Text Handling
		userText = v.Message.GetConversation()
		if userText == "" { userText = v.Message.GetExtendedTextMessage().GetText() }

		if userText != "" {
			fmt.Println("👀 [READ] Marked as Read")
			client.MarkRead(ctx, []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender)

			readDelay := len(userText) / 10
			if isActiveChat { readDelay = 1 } 
			if readDelay < 1 { readDelay = 1 }
			
			if interrupted := waitAndCheckOwner(ctx, chatID, readDelay); interrupted { return }
		}
	}

	if userText == "" { return }

	// =================================================
	// 🧠 STEP 3: GENERATE
	// =================================================
	
	rawBotID := client.Store.ID.User
	botID := strings.Split(rawBotID, ":")[0]
	botID = strings.Split(botID, "@")[0]

	inputType := "text"
	if v.Message.GetAudioMessage() != nil { inputType = "voice" }

	aiResponse := generateCloneReply(botID, chatID, userText, senderName, inputType)
	if aiResponse == "" { return }

	// =================================================
	// ✍️ STEP 4: TYPING & SENDING
	// =================================================

	fmt.Println("✍️ [TYPING] Composing...")
	client.SendChatPresence(ctx, v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	
	typeSpeed := len(aiResponse) / 7
	if isActiveChat { typeSpeed = len(aiResponse) / 12 }
	if typeSpeed < 2 { typeSpeed = 2 }

	if interrupted := waitAndCheckOwner(ctx, chatID, typeSpeed); interrupted { 
		client.SendChatPresence(ctx, v.Info.Chat, types.ChatPresencePaused, types.ChatPresenceMediaText)
		return 
	}

	// 🚀 SEND
	client.SendChatPresence(ctx, v.Info.Chat, types.ChatPresencePaused, types.ChatPresenceMediaText)
	sendCleanReply(client, v.Info.Chat, v.Info.ID, aiResponse)
	
	// Save AI Reply
	key := fmt.Sprintf(KeyChatHistory, botID, chatID)
	rdb.RPush(ctx, key, "Me: "+aiResponse)
	
	fmt.Printf("🚀 [SENT] %s\n", aiResponse)
	
	// 🌟 STEP 5: STAY ONLINE (Sticky Mode)
	go keepOnline(client, v.Info.Chat, chatID)
}

// 🛡️ HELPER: Sticky Online Status
func keepOnline(client *whatsmeow.Client, jid types.JID, chatID string) {
	ctx := context.Background()
	for i := 0; i < 6; i++ {
		time.Sleep(10 * time.Second)
		
		lastOwnerMsgStr, _ := rdb.Get(ctx, fmt.Sprintf(KeyLastOwnerMsg, chatID)).Result()
		if lastOwnerMsgStr != "" {
			var lastOwnerMsg int64
			fmt.Sscanf(lastOwnerMsgStr, "%d", &lastOwnerMsg)
			if time.Now().Unix() - lastOwnerMsg < 10 {
				return // Owner active, stop sticky mode
			}
		}
		client.SendPresence(ctx, types.PresenceAvailable)
	}
	client.SendPresence(ctx, types.PresenceUnavailable)
}

// 🛡️ HELPER: Wait while checking Owner
func waitAndCheckOwner(ctx context.Context, chatID string, seconds int) bool {
	for i := 0; i < seconds; i++ {
		lastOwnerMsgStr, _ := rdb.Get(ctx, fmt.Sprintf(KeyLastOwnerMsg, chatID)).Result()
		if lastOwnerMsgStr != "" {
			var lastOwnerMsg int64
			fmt.Sscanf(lastOwnerMsgStr, "%d", &lastOwnerMsg)
			if time.Now().Unix() - lastOwnerMsg < 5 {
				fmt.Println("🛑 [INTERRUPT] Owner is active! AI Aborting.")
				return true 
			}
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

// 🧬 CLONE ENGINE
func generateCloneReply(botID, chatID, currentMsg, senderName, inputType string) string {
	ctx := context.Background()
	historyList, _ := rdb.LRange(ctx, fmt.Sprintf(KeyChatHistory, botID, chatID), 0, -1).Result()
	history := strings.Join(historyList, "\n")

	voiceInstruction := ""
	if inputType == "voice" {
		voiceInstruction = "⚠️ NOTE: User sent a VOICE MESSAGE. The text above is the transcription."
	}

	fullPrompt := fmt.Sprintf(`
You are "Me" (The Owner). You are chatting with "%s".
Reply EXACTLY like "Me".

INSTRUCTIONS:
1. **Mimic Tone:** Copy my style from history (Short/Long/Funny/Rude).
2. **Context:** %s
3. **Closing:** Don't drag conversations. End them if natural.
4. **Humanity:** Behave like a human, not an AI.
5. **Wait:** If the conversation is slow, give short replies.

HISTORY:
%s
---
USER (%s): %s
ME:`, senderName, voiceInstruction, history, inputType, currentMsg)

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
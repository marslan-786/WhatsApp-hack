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
	KeyAutoAITargets = "autoai:targets_set"
	KeyChatHistory   = "chat:history:%s:%s" // botID:chatID
	KeyLastMsgTime   = "autoai:last_msg_time:%s"
	KeyLastOwnerMsg  = "autoai:last_owner_msg:%s"
)

// 📝 1. HISTORY RECORDER (Background)
func RecordChatHistory(client *whatsmeow.Client, v *events.Message, botID string) {
	// Anti-Crash: Ignore Old Messages
	if time.Since(v.Info.Timestamp) > 60*time.Second { return }
	
	if v.Info.IsGroup || strings.Contains(v.Info.Chat.String(), "@newsletter") || v.Info.Chat.String() == "status@broadcast" {
		return
	}

	// Run in background
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
			// Transcribe for history
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

		entry := fmt.Sprintf("%s: %s", senderName, text)
		key := fmt.Sprintf(KeyChatHistory, botID, chatID)
		rdb.RPush(ctx, key, entry)
		rdb.LTrim(ctx, key, -50, -1)
	}()
}

// 🚀 2. COMMAND HANDLER
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
		rdb.SAdd(ctx, KeyAutoAITargets, targetName)
		fmt.Printf("\n🔥 [AUTO-AI] ADDED TARGET: %s\n", targetName)
		sendCleanReply(client, v.Info.Chat, v.Info.ID, "✅ AI Active for: "+targetName)

	case "off":
		if len(args) < 2 {
			sendCleanReply(client, v.Info.Chat, v.Info.ID, "❌ Specify Name or 'all'.")
			return
		}
		targetName := strings.Join(args[1:], " ")
		if strings.ToLower(targetName) == "all" {
			rdb.Del(ctx, KeyAutoAITargets)
			sendCleanReply(client, v.Info.Chat, v.Info.ID, "🛑 Stopped for EVERYONE.")
		} else {
			rdb.SRem(ctx, KeyAutoAITargets, targetName)
			sendCleanReply(client, v.Info.Chat, v.Info.ID, "🛑 Stopped for: "+targetName)
		}

	case "list":
		targets, _ := rdb.SMembers(ctx, KeyAutoAITargets).Result()
		msg := "🤖 *Active Targets:*\n"
		for i, t := range targets {
			msg += fmt.Sprintf("%d. %s\n", i+1, t)
		}
		sendCleanReply(client, v.Info.Chat, v.Info.ID, msg)
	}
}

// 🧠 3. MAIN CHECKER (With Hard Logs)
func CheckAndHandleAutoReply(client *whatsmeow.Client, v *events.Message) bool {
	// Ignore old messages
	if time.Since(v.Info.Timestamp) > 60*time.Second { return false }
	if v.Info.IsFromMe { return false }

	ctx := context.Background()
	chatID := v.Info.Chat.String()

	// 🛑 OWNER INTERRUPT CHECK
	lastOwnerMsgStr, _ := rdb.Get(ctx, fmt.Sprintf(KeyLastOwnerMsg, chatID)).Result()
	if lastOwnerMsgStr != "" {
		var lastOwnerMsg int64
		fmt.Sscanf(lastOwnerMsgStr, "%d", &lastOwnerMsg)
		// If owner spoke in last 60s, don't trigger AI
		if time.Now().Unix() - lastOwnerMsg < 60 {
			// fmt.Println("🛑 Owner Active. AI Sleeping.")
			return false
		}
	}

	targets, err := rdb.SMembers(ctx, KeyAutoAITargets).Result()
	if err != nil || len(targets) == 0 { return false }

	incomingName := v.Info.PushName
	if incomingName == "" {
		if contact, err := client.Store.Contacts.GetContact(ctx, v.Info.Sender); err == nil && contact.Found {
			incomingName = contact.FullName
			if incomingName == "" { incomingName = contact.PushName }
		}
	}
	
	// 🔥 HARD DEBUG LOGS
	// fmt.Printf("🕵️ Checking: %s (Targets: %v)\n", incomingName, targets)

	incomingLower := strings.ToLower(incomingName)
	matchedTarget := ""

	for _, t := range targets {
		if strings.Contains(incomingLower, strings.ToLower(t)) {
			matchedTarget = t
			break
		}
	}

	if matchedTarget != "" {
		fmt.Printf("\n🔔 [AI MATCH] Target Found: %s (Detected: %s)\n", matchedTarget, incomingName)
		
		// اگر وائس ہے تو بتاؤ
		if v.Message.GetAudioMessage() != nil {
			fmt.Println("🎤 [DETECT] Voice Message Detected in Checker!")
		}

		go processAIResponse(client, v, incomingName)
		return true 
	}
	return false
}

// 🤖 4. AI BEHAVIOR ENGINE
func processAIResponse(client *whatsmeow.Client, v *events.Message, senderName string) {
	ctx := context.Background()
	chatID := v.Info.Chat.String()
	
	// ⏳ A. TIMING CALCULATION
	lastTimeStr, _ := rdb.Get(ctx, fmt.Sprintf(KeyLastMsgTime, chatID)).Result()
	var lastTime int64
	if lastTimeStr != "" {
		fmt.Sscanf(lastTimeStr, "%d", &lastTime)
	}
	
	// Update Time immediately
	currentTime := time.Now().Unix()
	rdb.Set(ctx, fmt.Sprintf(KeyLastMsgTime, chatID), fmt.Sprintf("%d", currentTime), 0)

	timeDiff := currentTime - lastTime
	isActiveChat := timeDiff < 60 // 1 Minute Rule

	// =================================================
	// 🎭 STEP 1: PHONE PICKUP (Cold Only)
	// =================================================
	
	if !isActiveChat {
		// COLD START: Wait 8-12s
		waitTime := 8 + rand.Intn(5)
		fmt.Printf("🐢 [MODE] Cold Start. Waiting %ds...\n", waitTime)
		
		if interrupted := waitAndCheckOwner(ctx, chatID, waitTime); interrupted { return }
		
		fmt.Println("📱 [STATUS] Coming Online...")
		client.SendPresence(ctx, types.PresenceAvailable)
		
	} else {
		// ACTIVE CHAT: Instant Online
		fmt.Println("⚡ [MODE] Active Chat (Instant).")
		client.SendPresence(ctx, types.PresenceAvailable)
	}

	// 🛑 Start Stickiness (Stay Online)
	stopSticky := make(chan bool)
	go keepOnline(client, v.Info.Chat, chatID, stopSticky)

	// =================================================
	// 👁️ STEP 2: READING / LISTENING
	// =================================================

	userText := ""
	
	// 🎤 Voice Handling (Detailed)
	if v.Message.GetAudioMessage() != nil {
		duration := int(v.Message.GetAudioMessage().GetSeconds())
		if duration == 0 { duration = 5 }

		fmt.Printf("🎤 [VOICE PROCESS] Duration: %ds\n", duration)
		
		// 1. Mark Read (Blue Tick) IMMEDIATELY
		client.MarkRead(ctx, []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender)
		
		// 2. Listen Delay (Simulate Listening)
		fmt.Println("🎧 [STATUS] Listening...")
		if interrupted := waitAndCheckOwner(ctx, chatID, duration); interrupted { 
			stopSticky <- true
			return 
		}

		// 3. Transcribe
		fmt.Println("🔄 [STATUS] Transcribing...")
		data, err := client.Download(ctx, v.Message.GetAudioMessage())
		if err == nil {
			transcribed, _ := TranscribeAudio(data)
			if transcribed != "" {
				userText = transcribed
				fmt.Printf("📝 [TRANSCRIPT] \"%s\"\n", userText)
			} else {
				userText = "[Unclear Voice Message]"
			}
		} else {
			userText = "[Voice Message Download Failed]"
		}

	} else {
		// 📝 Text Handling
		userText = v.Message.GetConversation()
		if userText == "" { userText = v.Message.GetExtendedTextMessage().GetText() }

		if userText != "" {
			// 1. Mark Read
			fmt.Println("👀 [READ] Marked Blue")
			client.MarkRead(ctx, []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender)

			// 2. Reading Delay
			readDelay := len(userText) / 10
			if isActiveChat { readDelay = 1 } 
			if readDelay < 1 { readDelay = 1 }
			
			if interrupted := waitAndCheckOwner(ctx, chatID, readDelay); interrupted { 
				stopSticky <- true
				return 
			}
		}
	}

	if userText == "" { 
		stopSticky <- true
		return 
	}

	// =================================================
	// 🧠 STEP 3: THINK & GENERATE
	// =================================================
	
	rawBotID := client.Store.ID.User
	botID := strings.Split(rawBotID, ":")[0]
	botID = strings.Split(botID, "@")[0]

	inputType := "text"
	if v.Message.GetAudioMessage() != nil { inputType = "voice" }

	aiResponse := generateCloneReply(botID, chatID, userText, senderName, inputType)
	if aiResponse == "" { 
		stopSticky <- true
		return 
	}

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
		stopSticky <- true
		return 
	}

	// 🚀 SEND
	client.SendChatPresence(ctx, v.Info.Chat, types.ChatPresencePaused, types.ChatPresenceMediaText)
	sendCleanReply(client, v.Info.Chat, v.Info.ID, aiResponse)
	
	key := fmt.Sprintf(KeyChatHistory, botID, chatID)
	rdb.RPush(ctx, key, "Me: "+aiResponse)
	
	fmt.Printf("🚀 [SENT] %s\n", aiResponse)
	
	// Stickiness will continue in background via keepOnline
	// We don't stop it here, let it run for its timeout
}

// 🛡️ HELPER: Sticky Online Status (Runs for 60s)
func keepOnline(client *whatsmeow.Client, jid types.JID, chatID string, stopChan chan bool) {
	ctx := context.Background()
	// fmt.Println("🌟 [STICKY] Starting Online Keep-Alive (60s)")
	
	for i := 0; i < 6; i++ { // 6 * 10s = 60s
		select {
		case <-stopChan:
			// fmt.Println("🛑 [STICKY] Stopped by process")
			return
		default:
			// Check Owner Interrupt
			lastOwnerMsgStr, _ := rdb.Get(ctx, fmt.Sprintf(KeyLastOwnerMsg, chatID)).Result()
			if lastOwnerMsgStr != "" {
				var lastOwnerMsg int64
				fmt.Sscanf(lastOwnerMsgStr, "%d", &lastOwnerMsg)
				if time.Now().Unix() - lastOwnerMsg < 5 {
					return // Owner active
				}
			}
			
			// Send Online
			client.SendPresence(ctx, types.PresenceAvailable)
			time.Sleep(10 * time.Second)
		}
	}
	
	fmt.Println("💤 [IDLE] 60s passed. Going Offline.")
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
		voiceInstruction = "⚠️ NOTE: User sent a VOICE MESSAGE. The text above is the transcription. Reply to the spoken content naturally."
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
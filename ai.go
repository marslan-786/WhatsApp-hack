package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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
	processAIConversation(client, v, query, cmd, false)
}

// 🧠 2. REPLY HANDLER
func handleAIReply(client *whatsmeow.Client, v *events.Message) bool {
	ext := v.Message.GetExtendedTextMessage()
	if ext == nil || ext.ContextInfo == nil || ext.ContextInfo.StanzaID == nil {
		return false
	}

	replyToID := ext.ContextInfo.GetStanzaID()
	senderID := v.Info.Sender.ToNonAD().String()

	if IsReplyToAI(senderID, replyToID) {
		userMsg := v.Message.GetConversation()
		if userMsg == "" {
			userMsg = v.Message.GetExtendedTextMessage().GetText()
		}

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

		processAIConversation(client, v, userMsg, "ai", true)
		return true
	}
	return false
}

// ⚙️ INTERNAL LOGIC
var (
	currentKeyID = 1
	keyMutex     sync.Mutex
)

// Custom API ka response structure
type CustomAPIResponse struct {
	Response string `json:"response"`
	Status   string `json:"status"`
	Error    string `json:"error"`
}

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

// Custom API Call Function
func callCustomAPI(apiURL string, prompt string) (string, error) {
	// Query param ko encode karein
	encodedPrompt := url.QueryEscape(prompt)
	fullURL := fmt.Sprintf("%s?message=%s", apiURL, encodedPrompt)

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 60 * time.Second} // 60 sec timeout taake browser load ho sake
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var apiResp CustomAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", err
	}

	if apiResp.Status == "failed" || apiResp.Error != "" {
		return "", fmt.Errorf("API Error: %s", apiResp.Error)
	}

	return apiResp.Response, nil
}

func processAIConversation(client *whatsmeow.Client, v *events.Message, query string, cmd string, isReply bool) {
	if !isReply {
		react(client, v.Info.Chat, v.Info.ID, "🧠")
	}

	senderID := v.Info.Sender.ToNonAD().String()
	history := GetAIHistory(senderID)

	aiName := "Impossible AI"
	if strings.ToLower(cmd) == "gpt" {
		aiName = "GPT-4"
	}

	// 🔥🔥🔥 TEXT AI PROMPT (Strict Script Matching) 🔥🔥🔥
	fullPrompt := fmt.Sprintf(
		"System: You are %s, a smart and friendly assistant.\n"+
			"🔴 TEXT MODE RULES (STRICT):\n"+
			"1. **DETECT SCRIPT:** Check the script of the 'User's New Message' carefully.\n"+
			"2. **MATCH SCRIPT:** \n"+
			"   - If User types in **ENGLISH**, reply in **ENGLISH**.\n"+
			"   - If User types in **ROMAN URDU** (e.g., 'kese ho'), reply in **ROMAN URDU**.\n"+
			"   - If User types in **URDU SCRIPT** (e.g., 'کیا حال ہے'), reply in **URDU SCRIPT**.\n"+
			"3. **NO HINDI SCRIPT:** Do NOT use Devanagari script (Hindi characters) under any circumstances in text mode.\n"+
			"4. **LENGTH:** Be natural, friendly, and detailed. No length restrictions.\n"+
			"----------------\n"+
			"Chat History (Ignore script here, focus on context):\n%s\n"+
			"----------------\n"+
			"User's New Message: %s\n"+
			"AI Response:",
		aiName, history, query)

	ctx := context.Background()
	var finalResponse string
	var lastError error
	var usedSource string = "CustomAPI"

	// =================================================================
	// 🚀 STEP 1: TRY CUSTOM API (Railway)
	// =================================================================
	
	// Yahan apni Railway wali API ka link dalein
	customURL := os.Getenv("CUSTOM_API_URL")
	if customURL == "" {
		// Fallback agar env set nahi hai
		customURL = "https://gemini-api-production-b665.up.railway.app/chat"
	}

	log.Println("🔄 Trying Custom API First...")
	customResp, err := callCustomAPI(customURL, fullPrompt)
	
	if err == nil && customResp != "" {
		finalResponse = customResp
		log.Println("✅ Custom API Success!")
	} else {
		log.Printf("⚠️ Custom API Failed (%v). Switching to Gemini Backup...", err)
		usedSource = "Gemini"
		
		// =================================================================
		// 🚀 STEP 2: FALLBACK TO GEMINI (Original Loop)
		// =================================================================
		
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

			// Note: Gemini API might truncate very long prompts, handle accordingly if needed
			result, err := genaiClient.Models.GenerateContent(ctx, "gemini-2.0-flash-exp", genai.Text(fullPrompt), nil)

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
	}

	// Agar dono fail ho jayen
	if finalResponse == "" {
		if !isReply {
			errMsg := "❌ System Overload. Custom API & All Backup Keys failed."
			if lastError != nil {
				errMsg += fmt.Sprintf(" (Last Error: %v)", lastError)
			}
			replyMessage(client, v, errMsg)
		}
		return
	}

	// Message Send Karna
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
		SaveAIHistory(senderID, query, finalResponse, respPtr.ID)
		if !isReply {
			// Reaction based on source (Optional - Debugging ke liye acha hai)
			if usedSource == "CustomAPI" {
				react(client, v.Info.Chat, v.Info.ID, "⚡") // Fast/Custom
			} else {
				react(client, v.Info.Chat, v.Info.ID, "🐢") // Backup/Gemini
			}
		}
	}
}
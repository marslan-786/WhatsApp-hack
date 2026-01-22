package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/genai"
	"google.golang.org/protobuf/proto"
)



// ⚙️ SETTINGS
const PY_SERVER = "http://localhost:5000"
const REMOTE_VOICE_URL = "https://voice-real-production.up.railway.app/speak"

func KeepServerAlive() {
	ticker := time.NewTicker(2 * time.Minute)
	go func() {
		for range ticker.C {
			http.Get(PY_SERVER)
			fmt.Println("💓 Ping sent to Python Server!")
		}
	}()
}

// 1️⃣ VOICE SELECTION
func HandleVoiceCommand(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) < 1 {
		replyMessage(client, v, "❌ Usage: .setvoice 1, .setvoice 2, etc.")
		return
	}
	voiceID := args[0]
	voiceFile := fmt.Sprintf("voice_%s.wav", strings.TrimSpace(voiceID))
	senderID := v.Info.Sender.ToNonAD().String()

	ctx := context.Background()
	rdb.Set(ctx, "user_voice_pref:"+senderID, voiceFile, 0)
	replyMessage(client, v, fmt.Sprintf("✅ Voice changed to: *Voice %s*", voiceID))
}

// 2️⃣ MAIN VOICE HANDLER
func HandleVoiceMessage(client *whatsmeow.Client, v *events.Message) {
	fmt.Println("🚀 AI Engine: Processing Voice...")
	audioMsg := v.Message.GetAudioMessage()
	if audioMsg == nil {
		return
	}
	senderID := v.Info.Sender.ToNonAD().String()

	replyContext := ""
	replyToID := ""
	
	if ctxInfo := v.Message.GetExtendedTextMessage().GetContextInfo(); ctxInfo != nil {
		replyToID = ctxInfo.GetStanzaID()
		if conv := ctxInfo.GetQuotedMessage().GetConversation(); conv != "" {
			replyContext = conv
		}
	} else if ctxInfo := v.Message.GetAudioMessage().GetContextInfo(); ctxInfo != nil {
		replyToID = ctxInfo.GetStanzaID()
	}

	isReplyToAI := IsReplyToAI(senderID, replyToID)
	if !isReplyToAI && replyToID != "" {
		fmt.Println("⚠️ Ignored: Reply is not to an AI message.")
	}

	client.SendChatPresence(context.Background(), v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaAudio)

	data, err := client.Download(context.Background(), audioMsg)
	if err != nil {
		fmt.Println("❌ Download Failed")
		return
	}

	userText, err := TranscribeAudio(data)
	if err != nil {
		return
	}
	fmt.Println("🗣️ User Said:", userText)

	if replyContext != "" {
		userText = fmt.Sprintf("(Reply to: '%s') %s", replyContext, userText)
	}

	aiResponse, _ := GetGeminiVoiceResponseWithHistory(userText, senderID)
	if aiResponse == "" {
		return
	}
	fmt.Println("🤖 AI Response:", aiResponse)

	rawAudio, err := GenerateVoice(aiResponse, senderID)
	if err != nil || len(rawAudio) == 0 {
		return
	}

	finalAudio, err := ConvertToOpus(rawAudio)
	if err != nil {
		finalAudio = rawAudio
	}

	up, err := client.Upload(context.Background(), finalAudio, whatsmeow.MediaAudio)
	if err != nil {
		return
	}

	resp, err := client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("audio/ogg; codecs=opus"),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
			FileLength:    proto.Uint64(uint64(len(finalAudio))),
			PTT:           proto.Bool(true),
		},
	})

	if err == nil && rdb != nil {
		SaveAIHistory(senderID, userText, aiResponse, resp.ID)
		fmt.Println("✅ Voice Note Sent!")
	}
}

// HELPER FUNCTIONS
func GenerateVoice(text string, senderID string) ([]byte, error) {
	fmt.Println("⚡ Sending Prompt to Python Server...")
	ctx := context.Background()
	voiceFile, err := rdb.Get(ctx, "user_voice_pref:"+senderID).Result()
	if err != nil || voiceFile == "" {
		voiceFile = "voice_1.wav"
	}
	return requestVoiceServer(REMOTE_VOICE_URL, text, voiceFile)
}

func requestVoiceServer(url string, text string, speakerFile string) ([]byte, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("text", text)
	writer.WriteField("speaker", speakerFile)
	writer.Close()

	client := http.Client{Timeout: 6000 * time.Second}
	resp, err := client.Post(url, writer.FormDataContentType(), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status: %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func GetGeminiVoiceResponseWithHistory(query string, senderID string) (string, string) {
	ctx := context.Background()
	history := GetAIHistory(senderID)

	var validKeys []string
	if mainKey := os.Getenv("GOOGLE_API_KEY"); mainKey != "" {
		validKeys = append(validKeys, mainKey)
	}
	for i := 1; i <= 50; i++ {
		if k := os.Getenv(fmt.Sprintf("GOOGLE_API_KEY_%d", i)); k != "" {
			validKeys = append(validKeys, k)
		}
	}

	// 🔥🔥🔥 ULTIMATE STRICT PROMPT FOR HINDI SCRIPT 🔥🔥🔥
	systemPrompt := fmt.Sprintf(`System: You are an AI that can ONLY write in Devanagari script (Hindi).
	
	🔴 CRITICAL RULE: YOU ARE FORBIDDEN FROM USING URDU SCRIPT (Nastaliq).
	🔴 CRITICAL RULE: Even if the user speaks Urdu, you MUST reply using HINDI CHARACTERS.

	Example 1:
	User: "Kya hal hai?"
	You: "मैं ठीक हूँ, तुम सुनाओ?"

	Example 2:
	User: "Kuch suna do yaar"
	You: "अरे यार, आज मौसम बहुत प्यारा है।"

	Example 3 (Complex):
	User: "Maza aa gaya"
	You: "हाँ यार, सच में मज़ा आ गया।"

	CONTEXT:
	- Tone: Friendly, casual, caring ('Yaar', 'Jaan').
	- Length: Keep it short (1-2 sentences) unless asked for a story/poem.
	- Language: Urdu/Hindi spoken language, BUT WRITTEN IN DEVANAGARI ONLY.

	Chat History: %s
	User Voice Message: "%s"`, history, query)

	// ... (Rest of the code remains same) ...
	
	// 1. Try Custom API
	customURL := os.Getenv("CUSTOM_API_URL")
	if customURL == "" {
		customURL = "https://gemini-api-production-b665.up.railway.app/chat"
	}

	encodedPrompt := url.QueryEscape(systemPrompt)
	apiReqURL := fmt.Sprintf("%s?message=%s", customURL, encodedPrompt)
	
	apiClient := &http.Client{Timeout: 90 * time.Second}
	resp, err := apiClient.Get(apiReqURL)

	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var apiResp struct {
			Response string `json:"response"`
			Status   string `json:"status"`
		}
		if json.Unmarshal(body, &apiResp) == nil && apiResp.Status == "success" {
			fmt.Println("✅ Voice Generated via Custom API (Hindi Script)!")
			return apiResp.Response, ""
		}
	} else {
		fmt.Printf("⚠️ Custom API Failed (%v). Switching to Backup...\n", err)
	}

	// 2. Try Gemini Keys
	for i, key := range validKeys {
		client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: key})
		if err != nil { continue }
		
		// Use flash model for speed
		resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash", genai.Text(systemPrompt), nil)
		if err != nil {
			fmt.Printf("❌ Key #%d Failed.\n", i+1)
			continue
		}
		return resp.Text(), ""
	}

	return "नेटवर्क का मसला है।", ""
}

func TranscribeAudio(audioData []byte) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "voice.ogg")
	part.Write(audioData)
	writer.Close()
	resp, err := http.Post(PY_SERVER+"/transcribe", writer.FormDataContentType(), body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct{ Text string `json:"text"` }
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Text, nil
}

func ConvertToOpus(inputData []byte) ([]byte, error) {
	inputFile := fmt.Sprintf("temp_in_%d.wav", time.Now().UnixNano())
	outputFile := fmt.Sprintf("temp_out_%d.ogg", time.Now().UnixNano())
	os.WriteFile(inputFile, inputData, 0644)
	defer os.Remove(inputFile)
	defer os.Remove(outputFile)
	cmd := exec.Command("ffmpeg", "-y", "-i", inputFile, "-c:a", "libopus", "-b:a", "16k", "-ac", "1", "-f", "ogg", outputFile)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(outputFile)
}
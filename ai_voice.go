package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"strings"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/genai"
)

// Python Server URL
const PY_SERVER = "http://localhost:5000"

// 🎤 ENTRY POINT: Jab user voice note bhejta hai
func HandleVoiceMessage(client *whatsmeow.Client, v *events.Message) {
	audioMsg := v.Message.GetAudioMessage()
	if audioMsg == nil {
		return
	}

	senderID := v.Info.Sender.ToNonAD().String()

	// 🎤 STATUS: "Recording audio..."
	stopRecording := make(chan bool)
	go func() {
		client.SendChatPresence(context.Background(), v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaAudio)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				client.SendChatPresence(context.Background(), v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaAudio)
			case <-stopRecording:
				client.SendChatPresence(context.Background(), v.Info.Chat, types.ChatPresencePaused, types.ChatPresenceMediaAudio)
				return
			}
		}
	}()
	defer func() { stopRecording <- true }()

	// 1. Download User's Voice
	data, err := client.Download(context.Background(), audioMsg)
	if err != nil {
		fmt.Println("❌ Download Failed:", err)
		return
	}

	// 2. Transcribe (User Voice -> Text)
	userText, err := TranscribeAudio(data)
	if err != nil || userText == "" {
		return
	}
	fmt.Println("🗣️ User Said:", userText)

	// 3. Gemini Brain (With History & 2.5 Flash)
	aiResponse, msgID := GetGeminiVoiceResponseWithHistory(userText, senderID)
	if aiResponse == "" {
		return
	}
	fmt.Println("🤖 AI Generated:", aiResponse)

	// 4. Generate Audio (AI Text -> Voice)
	refVoice := "voices/male_urdu.wav"
	audioBytes, err := GenerateVoice(aiResponse, refVoice)
	if err != nil {
		fmt.Println("❌ TTS Failed:", err)
		return
	}

	// 5. Send Audio back to WhatsApp
	up, err := client.Upload(context.Background(), audioBytes, whatsmeow.MediaAudio)
	if err != nil {
		return
	}

	resp, _ := client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			URL:           PtrString(up.URL),
			DirectPath:    PtrString(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      PtrString("audio/ogg; codecs=opus"),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
			FileLength:    PtrUint64(uint64(len(audioBytes))),
			PTT:           PtrBool(true), // Blue Mic
		},
	})

	// 💾 6. UPDATE REDIS HISTORY (Crucial Step)
	// اگر میسج چلا گیا ہے تو ہسٹری اپڈیٹ کریں تاکہ ٹیکسٹ چیٹ کو بھی یاد رہے
	if resp != nil && rdb != nil {
		UpdateAIHistory(senderID, userText, aiResponse, resp.ID)
	}
}

// 🧠 GEMINI WITH HISTORY + 2.5 FLASH + HINDI SCRIPT
func GetGeminiVoiceResponseWithHistory(query string, senderID string) (string, string) {
	ctx := context.Background()
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY_1")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		log.Println("Gemini Client Error:", err)
		return "माफ़ कीजिये, सिस्टम में कोई खराबी है।", ""
	}

	// 📜 FETCH HISTORY FROM REDIS
	var history string = ""
	if rdb != nil {
		key := "ai_session:" + senderID
		val, err := rdb.Get(ctx, key).Result()
		if err == nil {
			var session AISession
			// AISession struct ai.go main define hai, yahan use ho jayega
			_ = json.Unmarshal([]byte(val), &session)
			
			// صرف پچھلے 30 منٹ کی بات چیت یاد رکھے
			if time.Now().Unix()-session.LastUpdated < 1800 {
				history = session.History
			}
		}
	}

	// لمبی ہسٹری کو کاٹ دیں تاکہ ٹوکنز ضائع نہ ہوں
	if len(history) > 1000 {
		history = history[len(history)-1000:]
	}

	// 🔥 PROMPT (History + Hindi Script Instruction)
	systemPrompt := fmt.Sprintf(`System: You are a smart assistant participating in a voice conversation.
    
    🔴 RULES:
    1. **Format:** Output ONLY in HINDI SCRIPT (Devanagari) so the TTS engine can read it as Urdu.
    2. **Language:** Speak polite, natural Urdu (using words like 'aap', 'janab', 'theek').
    3. **Context:** Use the Chat History below to understand the conversation flow.
    4. **Length:** Keep it conversational and short (1-2 sentences).
    
    📜 Chat History:
    %s
    
    👤 User's New Voice Message: "%s"`, history, query)

	// ✅ Model set to 2.5 Flash as requested
	resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash", genai.Text(systemPrompt), nil)

	if err != nil {
		log.Println("Gemini Voice Error:", err)
		// Fallback Fallback logic for Key Rotation could be added here if needed
		// For now returning safe error in Hindi script
		return "माफ़ कीजिये, मुझे आपकी बात समझ नहीं आई।", ""
	}

	return resp.Text(), ""
}

// 💾 HISTORY UPDATER Helper
func UpdateAIHistory(senderID, userQuery, aiResponse, msgID string) {
	ctx := context.Background()
	key := "ai_session:" + senderID
	
	// پرانا ڈیٹا لائیں
	var history string
	val, err := rdb.Get(ctx, key).Result()
	if err == nil {
		var session AISession
		json.Unmarshal([]byte(val), &session)
		history = session.History
	}

	// نیا ڈیٹا جوڑیں
	// نوٹ: ہم ہسٹری میں بھی ہندی اسکرپٹ ہی محفوظ کر رہے ہیں، جو کہ ٹھیک ہے۔
	// Gemini اگلی بار اسے پڑھ کر سمجھ جائے گا کہ کیا بات ہوئی تھی۔
	newHistory := fmt.Sprintf("%s\nUser: %s\nAI: %s", history, userQuery, aiResponse)

	newSession := AISession{
		History:     newHistory,
		LastMsgID:   msgID,
		LastUpdated: time.Now().Unix(),
	}

	jsonData, _ := json.Marshal(newSession)
	rdb.Set(ctx, key, jsonData, 30*time.Minute)
}

// 🔌 HELPER: Go -> Python (Transcribe)
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

	var result struct {
		Text string `json:"text"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Text, nil
}

// 🔌 HELPER: Go -> Python (Speak)
func GenerateVoice(text string, refFile string) ([]byte, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	writer.WriteField("text", text)
	// 'hi' bhej rahe hain taake Devanagari script parh sake
	writer.WriteField("lang", "hi")

	fileData, err := os.ReadFile(refFile)
	if err != nil {
		return nil, err
	}
	part, _ := writer.CreateFormFile("speaker_wav", filepath.Base(refFile))
	part.Write(fileData)
	writer.Close()

	resp, err := http.Post(PY_SERVER+"/speak", writer.FormDataContentType(), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API Error: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return io.ReadAll(resp.Body)
}

// ✅ HELPER FUNCTIONS
func PtrString(s string) *string { return &s }
func PtrBool(b bool) *bool       { return &b }
func PtrUint64(i uint64) *uint64 { return &i }

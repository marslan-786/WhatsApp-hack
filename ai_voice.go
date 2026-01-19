package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
    "log"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
    "google.golang.org/genai" // ✅ Gemini Library Import
)

// Python Server URL
const PY_SERVER = "http://localhost:5000"

// 🎤 ENTRY POINT: Jab user voice note bhejta hai
func HandleVoiceMessage(client *whatsmeow.Client, v *events.Message) {
	audioMsg := v.Message.GetAudioMessage()
	if audioMsg == nil { return }

	// 🎤 STATUS: "Recording audio..." (تاکہ یوزر کو لگے کہ آپ بول رہے ہیں)
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
    // یہاں ہم Whisper کو کہیں گے کہ جو بھی سنے، اسے اردو سمجھے
	userText, err := TranscribeAudio(data)
	if err != nil || userText == "" {
		return
	}
    fmt.Println("🗣️ User Said:", userText)

	// 3. Gemini Brain (Text -> AI Response in Hindi Script)
	aiResponse := GetGeminiVoiceResponse(userText)
	if aiResponse == "" { return }
    fmt.Println("🤖 AI Generated (For TTS):", aiResponse)

	// 4. Generate Audio (AI Text -> Voice)
    // نوٹ: یہ text ہندی رسم الخط میں ہوگا لیکن XTTS اسے اردو لہجے میں پڑھے گا
	refVoice := "voices/male_urdu.wav" 
	audioBytes, err := GenerateVoice(aiResponse, refVoice)
	if err != nil {
        fmt.Println("❌ TTS Failed:", err)
		return
	}

	// 5. Send Audio back to WhatsApp (No Text Reply!)
	up, err := client.Upload(context.Background(), audioBytes, whatsmeow.MediaAudio)
	if err != nil { return }

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
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
}

// 🔌 HELPER: Go -> Python (Transcribe)
func TranscribeAudio(audioData []byte) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "voice.ogg")
	part.Write(audioData)
	writer.Close()

	resp, err := http.Post(PY_SERVER+"/transcribe", writer.FormDataContentType(), body)
	if err != nil { return "", err }
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
	// ⚠️ IMPORTANT: XTTS Urdu ko nahi janta, isliye hum 'hi' bhej rahe hain
    // Gemini humein text Hindi Script mein dega, isliye 'hi' engine usay sahi parhega.
	writer.WriteField("lang", "hi") 

	fileData, err := os.ReadFile(refFile)
	if err != nil { return nil, err }
	part, _ := writer.CreateFormFile("speaker_wav", filepath.Base(refFile))
	part.Write(fileData)
	writer.Close()

	resp, err := http.Post(PY_SERVER+"/speak", writer.FormDataContentType(), body)
	if err != nil { return nil, err }
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API Error: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return io.ReadAll(resp.Body)
}

// 🧠 SPECIAL GEMINI FOR VOICE (The Trick)
func GetGeminiVoiceResponse(query string) string {
    ctx := context.Background()
    // انوائرمنٹ سے کی اٹھائیں
    apiKey := os.Getenv("GOOGLE_API_KEY")
    if apiKey == "" {
        // Fallback: Cycle check (ai.go wala logic yahan simple rakha hai)
        apiKey = os.Getenv("GOOGLE_API_KEY_1") 
    }

    client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
    if err != nil {
        log.Println("Gemini Client Error:", err)
        return ""
    }

    // 🔥 THE MAGIC PROMPT 🔥
    // یہ پروموٹ Gemini کو مجبور کرے گا کہ وہ اردو بولے لیکن لکھے ہندی رسم الخط میں
    systemPrompt := `You are a helpful assistant. The user is speaking to you.
    
    🔴 CRITICAL INSTRUCTIONS FOR VOICE GENERATION:
    1. The user is speaking Urdu/Hindi.
    2. Your response will be converted to Audio by an engine that ONLY reads Hindi Script (Devanagari).
    3. **YOU MUST OUTPUT ONLY IN HINDI SCRIPT (DEVANAGARI).**
    4. **DO NOT** use Urdu Script (Nastaliq) and **DO NOT** use English/Roman.
    5. **Style:** Use polite and natural Urdu vocabulary (e.g., use 'Aap', 'Janab', 'Shukriya' instead of pure Hindi 'Dhanyavad' if possible, but keep it understandable).
    6. Keep the response short and conversational (1-2 sentences).
    
    User said: "` + query + `"`

    resp, err := client.Models.GenerateContent(ctx, "gemini-1.5-flash", genai.Text(systemPrompt), nil)
    if err != nil {
        log.Println("Gemini Voice Error:", err)
        // Fallback agar API fail ho:
        return "میں معذرت خواہ ہوں، مجھے کچھ سمجھ نہیں آیا۔" // یہ اردو سکرپٹ ہے، شاید TTS نہ پڑھے، لیکن یہ ایرر کیس ہے۔
    }

    return resp.Text()
}

// ✅ HELPER FUNCTIONS
func PtrString(s string) *string { return &s }
func PtrBool(b bool) *bool       { return &b }
func PtrUint64(i uint64) *uint64 { return &i }

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
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/genai"
)

// ⚙️ SETTINGS
const PY_SERVER = "http://localhost:5000"
const REMOTE_VOICE_URL = "https://voice-real-production.up.railway.app/speak"

// 🎤 MAIN HANDLER
func HandleVoiceMessage(client *whatsmeow.Client, v *events.Message) {
	fmt.Println("🚀 AI Engine: Processing Voice...")

	audioMsg := v.Message.GetAudioMessage()
	if audioMsg == nil {
		return
	}

	senderID := v.Info.Sender.ToNonAD().String()

	// 1. Check Reply Context (اگلے بندے نے کس بات پر جواب دیا؟)
	replyContext := ""
	quoted := v.Message.GetExtendedTextMessage().GetContextInfo().GetQuotedMessage()
	if quoted != nil {
		// اگر ٹیکسٹ پر ریپلائی ہے
		if conversation := quoted.GetConversation(); conversation != "" {
			replyContext = conversation
		} else if imageMsg := quoted.GetImageMessage(); imageMsg != nil {
			replyContext = imageMsg.GetCaption()
		} else if videoMsg := quoted.GetVideoMessage(); videoMsg != nil {
			replyContext = videoMsg.GetCaption()
		}
		// اگر وہ وائس نوٹ پر ریپلائی ہے تو ہم آڈیو نہیں سن سکتے، لیکن ہم اسے بتا دیں گے
		if quoted.GetAudioMessage() != nil {
			replyContext = "[User replied to a previous Voice Note]"
		}
	}

	// ⏳ Status: Recording Audio...
	client.SendChatPresence(context.Background(), v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaAudio)

	// 2. Download
	data, err := client.Download(context.Background(), audioMsg)
	if err != nil {
		fmt.Println("❌ Download Failed")
		return
	}

	// 3. Transcribe
	userText, err := TranscribeAudio(data)
	if err != nil {
		return
	}
	fmt.Println("🗣️ User Said:", userText)

	if replyContext != "" {
		fmt.Println("🔗 Reply Context Found:", replyContext)
		// یوزر کا میسج موڈیفائی کر دیں تاکہ سیاق و سباق شامل ہو جائے
		userText = fmt.Sprintf("(In reply to: '%s') %s", replyContext, userText)
	}

	// 4. Gemini Brain
	aiResponse, _ := GetGeminiVoiceResponseWithHistory(userText, senderID)
	if aiResponse == "" {
		return
	}
	fmt.Println("🤖 AI Response:", aiResponse)

	// 5. Generate Voice
	audioBytes, err := GenerateVoice(aiResponse)
	if err != nil || len(audioBytes) == 0 {
		return
	}

	// 6. Upload & Send (Correct OGG MimeType)
	up, err := client.Upload(context.Background(), audioBytes, whatsmeow.MediaAudio)
	if err != nil {
		return
	}

	_, err = client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			URL:           PtrString(up.URL),
			DirectPath:    PtrString(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      PtrString("audio/ogg; codecs=opus"), // ✅ Now actually correct!
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
			FileLength:    PtrUint64(uint64(len(audioBytes))),
			PTT:           PtrBool(true), // ✅ Shows as blue waveform
		},
	})

	if err == nil && rdb != nil {
		UpdateAIHistory(senderID, userText, aiResponse, "")
		fmt.Println("✅ Voice Note Sent!")
	}
}

// ... (Baqi Gemini, Transcribe, UpdateHistory functions same as before) ...
// (صرف GetGeminiVoiceResponseWithHistory اور GenerateVoice وہی رہیں گے جو پچھلی بار دیے تھے)
// (GenerateVoice فنکشن میں بس یہ دھیان رہے کہ وہ اب Python server کے نئے /speak اینڈ پوائنٹ کو ہٹ کرے گا)

// 🧠 GEMINI LOGIC (Modified for Hindi Script / Pure Urdu)
func GetGeminiVoiceResponseWithHistory(query string, senderID string) (string, string) {
	ctx := context.Background()

	// 🔥 DYNAMIC KEY LOADER (Auto-Discovery)
	// اب ہارڈ کوڈنگ کی ضرورت نہیں، یہ خود 1 سے 50 تک چیک کر لے گا
	var validKeys []string

	// 1. سب سے پہلے مین کی (Base Key) چیک کریں
	if mainKey := os.Getenv("GOOGLE_API_KEY"); mainKey != "" {
		validKeys = append(validKeys, mainKey)
	}

	// 2. اب لوپ لگا کر _1 سے _50 تک چیک کریں
	// اگر آپ نے بیچ میں کوئی نمبر چھوڑ بھی دیا (مثلاً 4 کے بعد سیدھا 10)، تو بھی یہ اسے ڈھونڈ لے گا
	for i := 1; i <= 50; i++ {
		keyName := fmt.Sprintf("GOOGLE_API_KEY_%d", i)
		if keyVal := os.Getenv(keyName); keyVal != "" {
			validKeys = append(validKeys, keyVal)
		}
	}

	// 🛑 اگر کوئی بھی Key نہیں ملی
	if len(validKeys) == 0 {
		fmt.Println("❌ Error: No GOOGLE_API_KEY found in environment variables!")
		return "سسٹم میں کوئی API Key موجود نہیں ہے۔", ""
	}

	fmt.Printf("ℹ️ Loaded %d API Keys for Rotation.\n", len(validKeys))

	// 🔄 RETRY LOOP (Keys Rotation)
	for i := 0; i < len(validKeys); i++ {
		currentKey := validKeys[i]
		fmt.Printf("🔑 AI Engine: Trying API Key #%d...\n", i+1)

		client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: currentKey})
		if err != nil {
			fmt.Println("⚠️ Client Error:", err)
			continue
		}

		var history string = ""
		if rdb != nil {
			key := "ai_session:" + senderID
			val, err := rdb.Get(ctx, key).Result()
			if err == nil {
				var session AISession
				_ = json.Unmarshal([]byte(val), &session)
				if time.Now().Unix()-session.LastUpdated < 3600 {
					history = session.History
				}
			}
		}
		if len(history) > 1500 {
			history = history[len(history)-1500:]
		}

		// 🔥 PROMPT (Hindi Script / Pure Urdu)
		systemPrompt := fmt.Sprintf(`System: You are a deeply caring, intimate friend.
		
		🔴 CRITICAL INSTRUCTIONS:
		1. **SCRIPT:** Output ONLY in **HINDI SCRIPT (Devanagari)**. Do NOT use Urdu/Arabic script.
		2. **LANGUAGE:** The actual language must be **PURE URDU**. 
		   - Use 'Muhabbat', 'Zindagi', 'Khayal', 'Pareshan'.
		3. **TONE:** Detect emotion. If user is sad, be very soft and comforting. If happy, be cheerful.
		4. **NO ROBOTIC SPEECH:** Speak fluently, like a real human. No formal headers.
		
		Chat History: %s
		User Voice: "%s"`, history, query)

		resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash", genai.Text(systemPrompt), nil)

		if err != nil {
			// اگر ایرر آئے تو اگلی Key ٹرائی کریں
			fmt.Printf("❌ Key #%d Failed: %v\n", i+1, err)
			fmt.Println("🔄 Switching to Next Key...")
			continue
		}

		fmt.Println("✅ Gemini Response Received!")
		return resp.Text(), ""
	}

	fmt.Println("❌ ALL API KEYS FAILED!")
	return "यार अभी मेरा नेट नहीं चल रहा।", ""
}

// 🔌 HELPER: Generate Voice (DIRECT & FAST)
func GenerateVoice(text string) ([]byte, error) {
	fmt.Println("⚡ Sending Full Prompt to 32-Core Server...")
	startTime := time.Now()

	// ہم سیدھا ایک ہی ریکویسٹ بھیج رہے ہیں (No Chunking)
	// 32 Cores اس کو سیکنڈوں میں ہینڈل کر لیں گے
	audio, err := requestVoiceServer(REMOTE_VOICE_URL, text)
	
	if err != nil {
		fmt.Println("❌ Remote Server Failed, trying Local...", err)
		// Local Fallback (gTTS)
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("text", text)
		writer.WriteField("lang", "hi") 
		writer.Close()
		resp, _ := http.Post("http://localhost:5000/speak", writer.FormDataContentType(), body)
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	}

	fmt.Printf("🏁 Full Voice Generated in %v\n", time.Since(startTime))
	return audio, nil
}

// 🔌 Network Helper (Standard)
func requestVoiceServer(url string, text string) ([]byte, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("text", text)
	writer.Close()

	// ٹائم آؤٹ بڑھا دیا ہے تاکہ بڑی فائل بھی آ سکے
	client := http.Client{Timeout: 300 * time.Second}
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

// 🔌 HELPER: Transcribe
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

// 💾 HISTORY
func UpdateAIHistory(senderID, userQuery, aiResponse, msgID string) {
	ctx := context.Background()
	key := "ai_session:" + senderID
	var history string
	val, err := rdb.Get(ctx, key).Result()
	if err == nil {
		var session AISession
		json.Unmarshal([]byte(val), &session)
		history = session.History
	}
	newHistory := fmt.Sprintf("%s\nUser: %s\nPartner: %s", history, userQuery, aiResponse)
	newSession := AISession{History: newHistory, LastMsgID: msgID, LastUpdated: time.Now().Unix()}
	jsonData, _ := json.Marshal(newSession)
	rdb.Set(ctx, key, jsonData, 60*time.Minute)
}

func PtrString(s string) *string { return &s }
func PtrBool(b bool) *bool       { return &b }
func PtrUint64(i uint64) *uint64 { return &i }
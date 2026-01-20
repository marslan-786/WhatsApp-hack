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
	"regexp"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/genai"
)

// ⚙️ SETTINGS
const PY_SERVER = "http://localhost:5000"
const USE_REMOTE_VOICE = true // ✅ TRUE = Use High Quality XTTS

// 🚀 VOICE SERVERS LIST
// آپ کا ریلوے کا پروجیکٹ لنک
var VoiceServers = []string{
	"https://voice-real-production.up.railway.app/speak", 
}

// 🎤 MAIN HANDLER
func HandleVoiceMessage(client *whatsmeow.Client, v *events.Message) {
	fmt.Println("🚀 AI Engine: Starting Voice Processing...")

	audioMsg := v.Message.GetAudioMessage()
	if audioMsg == nil {
		return
	}

	senderID := v.Info.Sender.ToNonAD().String()

	// ⏳ Typing/Recording Status
	stopRecording := make(chan bool)
	go func() {
		client.SendChatPresence(context.Background(), v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaAudio)
		ticker := time.NewTicker(4 * time.Second)
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

	// 1. Download
	fmt.Println("📥 AI Engine: Downloading Audio...")
	data, err := client.Download(context.Background(), audioMsg)
	if err != nil {
		fmt.Println("❌ Download Failed:", err)
		return
	}

	// 2. Transcribe
	fmt.Println("👂 AI Engine: Transcribing Audio...")
	userText, err := TranscribeAudio(data)
	if err != nil || userText == "" {
		fmt.Println("❌ Transcribe Failed:", err)
		return
	}
	fmt.Println("🗣️ User Said:", userText)

	// 3. Gemini Brain
	fmt.Println("🧠 AI Engine: Thinking (Hindi Script / Urdu Language)...")
	aiResponse, _ := GetGeminiVoiceResponseWithHistory(userText, senderID)

	if aiResponse == "" {
		return
	}
	// لاگ میں دیکھنے کے لیے کہ اس نے ہندی میں کیا لکھا ہے
	fmt.Println("🤖 AI Generated (Script):", aiResponse)

	// 4. Generate Voice
	fmt.Println("🎙️ AI Engine: Generating Voice Reply...")
	audioBytes, err := GenerateVoice(aiResponse)

	// ✅ SAFETY CHECK
	if err != nil || len(audioBytes) == 0 {
		fmt.Println("❌ TTS Failed (Empty File):", err)
		return
	}

	// 5. Send
	fmt.Println("📤 AI Engine: Uploading Voice Note...")
	up, err := client.Upload(context.Background(), audioBytes, whatsmeow.MediaAudio)
	if err != nil {
		return
	}

	resp, err := client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			URL:           PtrString(up.URL),
			DirectPath:    PtrString(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      PtrString("audio/ogg; codecs=opus"),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
			FileLength:    PtrUint64(uint64(len(audioBytes))),
			PTT:           PtrBool(true),
		},
	})

	if err == nil && rdb != nil {
		UpdateAIHistory(senderID, userText, aiResponse, resp.ID)
		fmt.Println("✅ AI Engine: Reply Sent Successfully!")
	}
}

// 🧠 GEMINI LOGIC (Modified for Hindi Script / Pure Urdu)
func GetGeminiVoiceResponseWithHistory(query string, senderID string) (string, string) {
	ctx := context.Background()

	apiKeys := []string{
		os.Getenv("GOOGLE_API_KEY"),
		os.Getenv("GOOGLE_API_KEY_1"),
		os.Getenv("GOOGLE_API_KEY_2"),
		os.Getenv("GOOGLE_API_KEY_3"),
		os.Getenv("GOOGLE_API_KEY_4"),
		os.Getenv("GOOGLE_API_KEY_5"),
        // ... (Add all your keys here as before)
	}

	var validKeys []string
	for _, k := range apiKeys {
		if k != "" {
			validKeys = append(validKeys, k)
		}
	}

	if len(validKeys) == 0 {
		return "سسٹم میں کوئی API Key موجود نہیں ہے۔", ""
	}

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

		// 🔥🔥🔥 CRITICAL PROMPT UPDATE 🔥🔥🔥
		systemPrompt := fmt.Sprintf(`System: You are a deeply caring, intimate friend.
		
		🔴 CRITICAL INSTRUCTIONS:
		1. **SCRIPT:** Output ONLY in **HINDI SCRIPT (Devanagari)**. Do NOT use Urdu/Arabic script.
		2. **LANGUAGE:** The actual language must be **PURE URDU**. 
		   - Use 'Muhabbat' (not 'Prem').
		   - Use 'Koshish' (not 'Prayas').
		   - Use 'Zindagi' (not 'Jeevan').
		3. **TONE & EMOTION:** - Detect the user's emotion immediately. If they are sad, be soft, slow, and comforting. If happy, be excited.
		   - Speak naturally like a human friend. No robot vibes.
		4. **PROHIBITED WORDS:** NEVER use 'Janab', 'Huzoor', 'Junoob', or formal bookish Urdu. Use casual 'Yaar', 'Jaan', 'Bhai'.
		5. **FLOW:** Write in short, conversational sentences with natural pauses. Don't make it a speech.

		Example:
		User (Urdu): "Mera dil udaas hai."
		You (Hindi Output): "अरे मेरी जान, क्या हुआ? उदास क्यों हो? मैं हूँ ना तुम्हारे साथ, मुझे बताओ क्या बात है।"

		Chat History: %s
		User Voice: "%s"`, history, query)

		resp, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash", genai.Text(systemPrompt), nil)

		if err != nil {
			fmt.Printf("❌ Key #%d Failed: %v\n", i+1, err)
			fmt.Println("🔄 Switching to Next Key...")
			continue
		}

		fmt.Println("✅ Gemini Response Received!")
		return resp.Text(), ""
	}

	fmt.Println("❌ ALL API KEYS FAILED!")
	return "यार मेरा दिमाग अभी काम नहीं कर रहा, थोड़ी देर बाद बात करते हैं।", "" // Fallback in Hindi script
}

// 🔌 HELPER: Generate Voice
func GenerateVoice(text string) ([]byte, error) {

	if USE_REMOTE_VOICE && len(VoiceServers) > 0 {
		fmt.Println("⚡ Starting Parallel Voice Generation (XTTS)...")
		startTime := time.Now()

		re := regexp.MustCompile(`[۔.?!।]+`) // Added Hindi Purnviram (।)
		rawParts := re.Split(text, -1)
		var chunks []string
		for _, s := range rawParts {
			if len(s) > 2 {
				chunks = append(chunks, s)
			}
		}
		if len(chunks) == 0 {
			chunks = []string{text}
		}

		fmt.Printf("📦 Splitting into %d chunks across %d servers...\n", len(chunks), len(VoiceServers))

		var wg sync.WaitGroup
		audioParts := make(map[int][]byte)
		var mu sync.Mutex

		for i, chunk := range chunks {
			wg.Add(1)
			serverIndex := i % len(VoiceServers)
			serverURL := VoiceServers[serverIndex]

			go func(idx int, txt string, url string) {
				defer wg.Done()
				fmt.Printf("🚀 Sending Chunk %d to %s...\n", idx, url)

				audio, err := requestVoiceServer(url, txt)
				if err == nil {
					mu.Lock()
					audioParts[idx] = audio
					mu.Unlock()
				} else {
					fmt.Printf("❌ Chunk %d Failed: %v\n", idx, err)
				}
			}(i, chunk, serverURL)
		}

		wg.Wait()

		var finalAudio []byte
		for i := 0; i < len(chunks); i++ {
			if part, ok := audioParts[i]; ok {
				if i == 0 {
					finalAudio = append(finalAudio, part...)
				} else {
					if len(part) > 44 {
						finalAudio = append(finalAudio, part[44:]...)
					}
				}
			}
		}

		fmt.Printf("🏁 Voice Generated in %v\n", time.Since(startTime))
		return finalAudio, nil
	}

	// Local Fallback
	fmt.Println("🏠 Generating Locally (gTTS Fallback)...")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("text", text)
	writer.WriteField("lang", "hi") // Local gTTS also supports Hindi
	writer.Close()

	resp, err := http.Post("http://localhost:5000/speak", writer.FormDataContentType(), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// 🔌 Network Helper
func requestVoiceServer(url string, text string) ([]byte, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("text", text)
	writer.Close()

	// High timeout for CPU generation
	client := http.Client{Timeout: 600 * time.Second}
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
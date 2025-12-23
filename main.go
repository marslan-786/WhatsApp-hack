package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/showwin/speedtest-go/speedtest"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	waLog "go.mau.fi/whatsmeow/util/log"
	waProto "go.mau.fi/whatsmeow/binary/proto"
)




func sendToolCard(client *whatsmeow.Client, v *events.Message, title, tool, info string) {
	card := fmt.Sprintf(`╔══════════════════════╗
║ ✨ %s ✨
╠══════════════════════╣
║ 🛠️ Tool: %s
║ 🚦 Status: Active
╠══════════════════════╣
║ ⚡ Power: 32GB RAM (Live)
╚══════════════════════╝
%s`, strings.ToUpper(title), tool, info)
	replyMessage(client, v, card)
}


func handleAI(client *whatsmeow.Client, v *events.Message, query string, cmd string) {
	if query == "" {
		replyMessage(client, v, "⚠️ Please provide a prompt.")
		return
	}
	react(client, v.Info.Chat, v.Info.ID, "🧠")

	
	aiName := "Impossible AI"
	if strings.ToLower(cmd) == "gpt" { aiName = "GPT-4" }
	systemInstructions := fmt.Sprintf("You are %s. Respond in the user's language. Be brief and professional.", aiName)

	
	
	models := []string{"openai", "mistral"}
	
	var finalResponse string
	success := false

	for _, model := range models {
		apiUrl := fmt.Sprintf("https://text.pollinations.ai/%s?model=%s&system=%s", 
			url.QueryEscape(query), model, url.QueryEscape(systemInstructions))

		resp, err := http.Get(apiUrl)
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
		replyMessage(client, v, "🤖 *Impossible AI:* All neural nodes are currently congested. Please try later.")
		return
	}
	
	replyMessage(client, v, finalResponse)
	react(client, v.Info.Chat, v.Info.ID, "✅")
}

func handleImagine(client *whatsmeow.Client, v *events.Message, prompt string) {
	if prompt == "" {
		replyMessage(client, v, "⚠️ Please provide a prompt.")
		return
	}
	react(client, v.Info.Chat, v.Info.ID, "🎨")

	imageUrl := fmt.Sprintf("https://image.pollinations.ai/prompt/%s?width=1024&height=1024&nologo=true", url.QueryEscape(prompt))
	
	resp, err := http.Get(imageUrl)
	if err != nil { return }
	defer resp.Body.Close()
	
	imgData, _ := io.ReadAll(resp.Body)

	up, err := client.Upload(context.Background(), imgData, whatsmeow.MediaImage)
	if err != nil { return }

	
	finalMsg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("image/jpeg"),
			Caption:       proto.String("✨ *Impossible AI Art:* " + prompt),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
			FileLength:    proto.Uint64(uint64(len(imgData))), 
		},
	}

	client.SendMessage(context.Background(), v.Info.Chat, finalMsg)
	react(client, v.Info.Chat, v.Info.ID, "✅")
}


func handleServerStats(client *whatsmeow.Client, v *events.Message) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	used := m.Alloc / 1024 / 1024
	sys := m.Sys / 1024 / 1024
	numCPU := runtime.NumCPU()
	goRoutines := runtime.NumGoroutine()

	stats := fmt.Sprintf(`╔══════════════════════╗
║     🖥️ SYSTEM DASHBOARD    
╠══════════════════════╣
║ 🚀 RAM Used: %d MB
║ 💎 Total RAM: 32 GB
║ 🧬 System Memory: %d MB
║ 🧠 CPU Cores: %d
║ 🧵 Active Threads: %d
║ 🟢 Status: Invincible
╚══════════════════════╝`, used, sys, numCPU, goRoutines)
	replyMessage(client, v, stats)
}



func handleSpeedTest(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "🚀")
	
	
	replyMessage(client, v, "📡 *Impossible Engine:* Analyzing network uplink...")

	
	var speedClient = speedtest.New()
	
	
	serverList, err := speedClient.FetchServers()
	if err != nil {
		replyMessage(client, v, "❌ Failed to fetch speedtest servers.")
		return
	}
	
	targets, _ := serverList.FindServer([]int{})
	if len(targets) == 0 {
		replyMessage(client, v, "❌ No reachable network nodes found.")
		return
	}

	
	s := targets[0]
	s.PingTest(nil)
	s.DownloadTest()
	s.UploadTest()

	
	result := fmt.Sprintf("╭─── 🚀 *NETWORK ANALYSIS* ───╮\n"+
		"│\n"+
		"│ 📡 *Node:* %s\n"+
		"│ 📍 *Location:* %s\n"+
		"│ ┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈\n"+
		"│ ⚡ *Latency:* %s\n"+
		"│ 📥 *Download:* %.2f Mbps\n"+
		"│ 📤 *Upload:* %.2f Mbps\n"+
		"│\n"+
		"╰────────────────────╯",
		s.Name, s.Country, s.Latency, s.DLSpeed, s.ULSpeed)

	
	replyMessage(client, v, result)
	react(client, v.Info.Chat, v.Info.ID, "✅")
}



type ReminiResponse struct {
	Status string `json:"status"`
	URL    string `json:"url"`
}


func uploadToTempHost(data []byte, filename string) (string, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("fileToUpload", filename)
	part.Write(data)
	writer.WriteField("reqtype", "fileupload")
	writer.Close()

	req, _ := http.NewRequest("POST", "https://catbox.moe/user/api.php", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return string(respBody), nil
}

func handleRemini(client *whatsmeow.Client, v *events.Message) {
	
	extMsg := v.Message.GetExtendedTextMessage()
	if extMsg == nil || extMsg.ContextInfo == nil || extMsg.ContextInfo.QuotedMessage == nil {
		replyMessage(client, v, "⚠️ Please reply to an image with *.remini*")
		return
	}

	quotedMsg := extMsg.ContextInfo.QuotedMessage
	imgMsg := quotedMsg.GetImageMessage()
	if imgMsg == nil {
		replyMessage(client, v, "⚠️ The replied message is not an image.")
		return
	}

	react(client, v.Info.Chat, v.Info.ID, "✨")
	
	
	imgData, err := client.Download(context.Background(), imgMsg)
	if err != nil {
		replyMessage(client, v, "❌ Failed to download original image.")
		return
	}

	
	
	publicURL, err := uploadToTempHost(imgData, "image.jpg")
	if err != nil || !strings.HasPrefix(publicURL, "http") {
		replyMessage(client, v, "❌ Failed to generate public link for processing.")
		return
	}

	
	apiURL := fmt.Sprintf("https://final-enhanced-production.up.railway.app/enhance?url=%s", url.QueryEscape(publicURL))
	resp, err := http.Get(apiURL)
	if err != nil {
		replyMessage(client, v, "❌ AI Enhancement Engine is offline.")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var reminiResp ReminiResponse
	json.Unmarshal(body, &reminiResp)

	if reminiResp.Status != "success" || reminiResp.URL == "" {
		replyMessage(client, v, "❌ AI failed to enhance image. Try another one.")
		return
	}

	
	
	enhancedResp, err := http.Get(reminiResp.URL)
	if err != nil { return }
	defer enhancedResp.Body.Close()

	fileName := fmt.Sprintf("remini_%d.jpg", time.Now().UnixNano())
	outFile, err := os.Create(fileName)
	if err != nil { return }
	io.Copy(outFile, enhancedResp.Body)
	outFile.Close()

	
	finalData, err := os.ReadFile(fileName)
	if err != nil { return }
	defer os.Remove(fileName)

	
	up, err := client.Upload(context.Background(), finalData, whatsmeow.MediaImage)
	if err != nil {
		replyMessage(client, v, "❌ Failed to send enhanced image.")
		return
	}

	finalMsg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:        proto.String(up.URL),
			DirectPath: proto.String(up.DirectPath),
			MediaKey:   up.MediaKey,
			Mimetype:   proto.String("image/jpeg"),
			Caption:    proto.String("✅ *Enhanced with Remini AI*"),
			FileSHA256: up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
			FileLength: proto.Uint64(uint64(len(finalData))),
		},
	}

	client.SendMessage(context.Background(), v.Info.Chat, finalMsg)
	react(client, v.Info.Chat, v.Info.ID, "✅")
}


func handleScreenshot(client *whatsmeow.Client, v *events.Message, targetUrl string) {
	if targetUrl == "" {
		replyMessage(client, v, "⚠️ *Usage:* .ss [Link]")
		return
	}
	react(client, v.Info.Chat, v.Info.ID, "📸")
	sendToolCard(client, v, "Web Capture", "Headless-Mobile", "🌐 Rendering: "+targetUrl)

	
	
	apiURL := fmt.Sprintf("https://api.screenshotmachine.com/?key=54be93&device=phone&dimension=1290x2796&url=%s", url.QueryEscape(targetUrl))

	
	resp, err := http.Get(apiURL)
	if err != nil {
		replyMessage(client, v, "❌ Screenshot engine failed to connect.")
		return
	}
	defer resp.Body.Close()

	
	fileName := fmt.Sprintf("ss_%d.jpg", time.Now().UnixNano())
	out, err := os.Create(fileName)
	if err != nil { return }
	
	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil { return }

	
	fileData, err := os.ReadFile(fileName)
	if err != nil { return }
	defer os.Remove(fileName) 

	
	up, err := client.Upload(context.Background(), fileData, whatsmeow.MediaImage)
	if err != nil {
		replyMessage(client, v, "❌ WhatsApp rejected the media upload.")
		return
	}

	
	finalMsg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:        proto.String(up.URL),
			DirectPath: proto.String(up.DirectPath),
			MediaKey:   up.MediaKey,
			Mimetype:   proto.String("image/jpeg"),
			Caption:    proto.String("✅ *Web Capture Success*\n🌐 " + targetUrl),
			FileSHA256: up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
			FileLength: proto.Uint64(uint64(len(fileData))),
		},
	}

	client.SendMessage(context.Background(), v.Info.Chat, finalMsg)
	react(client, v.Info.Chat, v.Info.ID, "✅")
}


func handleWeather(client *whatsmeow.Client, v *events.Message, city string) {
	if city == "" { city = "Okara" }
	react(client, v.Info.Chat, v.Info.ID, "🌦️")
	
	
	apiUrl := "https://api.wttr.in/" + url.QueryEscape(city) + "?format=3"
	resp, _ := http.Get(apiUrl)
	data, _ := io.ReadAll(resp.Body)
	
	msg := fmt.Sprintf("🌦️ *Live Weather Report:* \n\n%s\n\nGenerated via Satellite-Impossible", string(data))
	replyMessage(client, v, msg)
}


func handleFancy(client *whatsmeow.Client, v *events.Message, text string) {
	if text == "" {
		replyMessage(client, v, "⚠️ Please provide text.\nExample: .fancy Nothing Is Impossible")
		return
	}

	
	styles := []struct { Name string; A rune; a rune }{
		{"Fraktur", 0x1D504, 0x1D51E},            
		{"Fraktur Bold", 0x1D56C, 0x1D586},       
		{"Math Bold", 0x1D400, 0x1D41A},          
		{"Math Italic", 0x1D434, 0x1D44E},        
		{"Math Bold Italic", 0x1D468, 0x1D482},   
		{"Script", 0x1D49C, 0x1D4B6},             
		{"Script Bold", 0x1D4D0, 0x1D4EA},        
		{"Double Struck", 0x1D538, 0x1D552},      
		{"Sans Serif", 0x1D5A0, 0x1D5BA},         
		{"Sans Bold", 0x1D5D4, 0x1D5EE},          
		{"Sans Italic", 0x1D608, 0x1D622},        
		{"Sans Bold Italic", 0x1D63C, 0x1D656},   
		{"Monospace", 0x1D670, 0x1D68A},          
		{"Circled White", 0x24B6, 0x24D0},       
		{"Circled Black", 0x1F150, 0x1F150},     
		{"Squared White", 0x1F130, 0x1F130},     
		{"Squared Black", 0x1F170, 0x1F170},     
		{"Fullwidth", 0xFF21, 0xFF41},            
		{"Modern Sans", 0x1D5A0, 0x1D5BA},        
		{"Gothic", 0x1D504, 0x1D51E},             
		{"Outline", 0x1D538, 0x1D552},            
		{"Math Serif Bold", 0x1D400, 0x1D41A},    
		{"Italic Serif", 0x1D434, 0x1D44E},       
		{"Bold Script", 0x1D4D0, 0x1D4EA},        
		{"Classic Gothic", 0x1D504, 0x1D51E},     
		{"Typewriter", 0x1D670, 0x1D68A},         
		{"Bold Sans", 0x1D5D4, 0x1D5EE},          
		{"Struck", 0x1D538, 0x1D552},             
		{"Small Caps Style", 0x1D400, 0x1D41A},   
		{"Fancy VIP", 0x1D4D0, 0x1D4EA},          
	}

	
	card := "╔════════════════════════╗\n"
	card += "║      ✨ *FANCY ENGINE V4* ✨     ║\n"
	card += "╠════════════════════════╣\n"
	card += "║ ⚡ *Power:* 32GB RAM VIP Server ║\n"
	card += "╚════════════════════════╝\n\n"

	
	for i, style := range styles {
		formatted := ""
		for _, char := range text {
			if char >= 'A' && char <= 'Z' {
				formatted += string(style.A + (char - 'A'))
			} else if char >= 'a' && char <= 'z' {
				
				if style.a == style.A {
					formatted += string(style.A + (char - 'a'))
				} else {
					formatted += string(style.a + (char - 'a'))
				}
			} else {
				formatted += string(char)
			}
		}
		card += fmt.Sprintf("【 %02d 】 %s\n", i+1, formatted)
	}

	
	card += "\n╔════════════════════════╗\n"
	card += "   👑 *ℑ𝔪𝔭𝔬𝔰𝔰𝔦𝔟𝔩𝔢 𝔅𝔬𝔱 𝔖𝔭𝔢𝔠𝔦𝔞𝔩*\n"
	card += "   🔥 _Scientists are now burning..._\n"
	card += "╚════════════════════════╝"

	replyMessage(client, v, card)
}


func handleDouyin(client *whatsmeow.Client, v *events.Message, url string) {
	if url == "" { replyMessage(client, v, "⚠️ Please provide a Douyin link."); return }
	react(client, v.Info.Chat, v.Info.ID, "🐉")
	sendPremiumCard(client, v, "Douyin", "Douyin-HQ", "🐉 Fetching Chinese TikTok content...")
	
	go downloadAndSend(client, v, url, "video")
}


func handleKwai(client *whatsmeow.Client, v *events.Message, url string) {
	if url == "" { replyMessage(client, v, "⚠️ Please provide a Kwai link."); return }
	react(client, v.Info.Chat, v.Info.ID, "🎞️")
	sendPremiumCard(client, v, "Kwai", "Kwai-Engine", "🎞️ Processing Kwai short video...")
	go downloadAndSend(client, v, url, "video")
}


func handleGoogle(client *whatsmeow.Client, v *events.Message, query string) {
	if query == "" {
		replyMessage(client, v, "⚠️ *Usage:* .google [query]")
		return
	}
	react(client, v.Info.Chat, v.Info.ID, "🔍")
	replyMessage(client, v, "📡 *Impossible Engine:* Scouring the web for '"+query+"'...")

	
	
	searchUrl := "https://duckduckgo.com/html/?q=" + url.QueryEscape(query)
	
	resp, err := http.Get(searchUrl)
	if err != nil {
		replyMessage(client, v, "❌ Search engine failed to respond.")
		return
	}
	defer resp.Body.Close()

	
	body, _ := io.ReadAll(resp.Body)
	htmlContent := string(body)

	
	menuText := "╭─── 🧐 *IMPOSSIBLE SEARCH* ───╮\n│\n"
	
	
	links := strings.Split(htmlContent, "class=\"result__a\" href=\"")
	
	count := 0
	for i := 1; i < len(links); i++ {
		if count >= 5 { break }
		
		
		linkPart := strings.Split(links[i], "\"")
		if len(linkPart) < 2 { continue }
		actualLink := linkPart[0]
		
		titlePart := strings.Split(links[i], ">")
		if len(titlePart) < 2 { continue }
		actualTitle := strings.Split(titlePart[1], "</a")[0]

		
		menuText += fmt.Sprintf("📍 *[%d]* %s\n│ 🔗 %s\n│ ┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈\n", count+1, actualTitle, actualLink)
		count++
	}

	if count == 0 {
		replyMessage(client, v, "❌ No results found. Try a different query.")
		return
	}

	menuText += "│\n╰────────────────────╯"
	replyMessage(client, v, menuText)
	react(client, v.Info.Chat, v.Info.ID, "✅")
}



func handleToPTT(client *whatsmeow.Client, v *events.Message) {
	
	var quoted *waProto.Message
	if extMsg := v.Message.GetExtendedTextMessage(); extMsg != nil && extMsg.ContextInfo != nil {
		quoted = extMsg.ContextInfo.QuotedMessage
	}

	
	if quoted == nil || (quoted.AudioMessage == nil && quoted.VideoMessage == nil) {
		replyMessage(client, v, "❌ Please reply to an audio or video file with *.toptt*")
		return
	}

	react(client, v.Info.Chat, v.Info.ID, "🎙️")
	
	
	var media whatsmeow.DownloadableMessage
	if quoted.AudioMessage != nil {
		media = quoted.AudioMessage
	} else {
		media = quoted.VideoMessage
	}

	data, err := client.Download(context.Background(), media)
	if err != nil {
		replyMessage(client, v, "❌ Failed to download media.")
		return
	}

	
	input := fmt.Sprintf("temp_in_%d", time.Now().UnixNano())
	output := fmt.Sprintf("temp_out_%d.opus", time.Now().UnixNano()) 
	os.WriteFile(input, data, 0644)

	
	
	
	
	
	cmd := exec.Command("ffmpeg", "-i", input, "-vn", "-c:a", "libopus", "-b:a", "16k", "-ac", "1", "-f", "ogg", output)
	err = cmd.Run()
	if err != nil {
		replyMessage(client, v, "❌ Conversion failed. Check if FFmpeg is installed.")
		os.Remove(input)
		return
	}

	
	pttData, _ := os.ReadFile(output)
	up, err := client.Upload(context.Background(), pttData, whatsmeow.MediaAudio)
	if err != nil { return }

	
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("audio/ogg; codecs=opus"), 
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
			FileLength:    proto.Uint64(uint64(len(pttData))),
			PTT:           proto.Bool(true), 
		},
	})

	
	os.Remove(input)
	os.Remove(output)
	react(client, v.Info.Chat, v.Info.ID, "✅")
}


func handleRemoveBG(client *whatsmeow.Client, v *events.Message) {
	extMsg := v.Message.GetExtendedTextMessage()
	if extMsg == nil || extMsg.ContextInfo == nil || extMsg.ContextInfo.QuotedMessage == nil {
		replyMessage(client, v, "⚠️ Please reply to an image with *.removebg*")
		return
	}

	quotedMsg := extMsg.ContextInfo.QuotedMessage
	imgMsg := quotedMsg.GetImageMessage()
	if imgMsg == nil {
		replyMessage(client, v, "⚠️ The replied message is not an image.")
		return
	}

	react(client, v.Info.Chat, v.Info.ID, "✂️")
	replyMessage(client, v, "🪄 *Impossible Engine:* Carving out the subject...")

	imgData, err := client.Download(context.Background(), imgMsg)
	if err != nil { return }

	inputPath := fmt.Sprintf("in_%d.jpg", time.Now().UnixNano())
	outputPath := fmt.Sprintf("out_%d.png", time.Now().UnixNano())
	os.WriteFile(inputPath, imgData, 0644)

	
	
	cmd := exec.Command("rembg", "i", inputPath, outputPath)
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		replyMessage(client, v, fmt.Sprintf("❌ *Engine Error:* \n%s", string(output)))
		os.Remove(inputPath)
		return
	}

	finalData, err := os.ReadFile(outputPath)
	if err != nil { return }

	defer os.Remove(inputPath)
	defer os.Remove(outputPath)

	up, err := client.Upload(context.Background(), finalData, whatsmeow.MediaImage)
	if err != nil { return }

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("image/png"),
			Caption:       proto.String("✅ *Background Removed Locally*"),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
			FileLength:    proto.Uint64(uint64(len(finalData))),
		},
	})
	react(client, v.Info.Chat, v.Info.ID, "✅")
}


func handleSteam(client *whatsmeow.Client, v *events.Message, url string) {
	if url == "" { return }
	react(client, v.Info.Chat, v.Info.ID, "🎮")
	sendPremiumCard(client, v, "Steam Media", "Steam-Engine", "🎮 Fetching official game trailer...")
	go downloadAndSend(client, v, url, "video")
}


func handleMega(client *whatsmeow.Client, v *events.Message, urlStr string) {
	if urlStr == "" { return }
	
	react(client, v.Info.Chat, v.Info.ID, "🚀")
	sendPremiumCard(client, v, "Mega Downloader", "Universal-Core", "🚀 Extracting encrypted stream...")

	go func() {
		tempDir := fmt.Sprintf("mega_%d", time.Now().UnixNano())
		os.Mkdir(tempDir, 0755)
		defer os.RemoveAll(tempDir)

		cmd := exec.Command("megadl", "--no-progress", "--path="+tempDir, urlStr)
		output, err := cmd.CombinedOutput()
		
		if err != nil {
			replyMessage(client, v, "❌ *Mega Error:* Invalid link or file too large.\nDetails: " + string(output))
			return
		}

		files, _ := os.ReadDir(tempDir)
		if len(files) == 0 {
			replyMessage(client, v, "❌ *Error:* File vanished during extraction.")
			return
		}
		
		fileName := files[0].Name()
		filePath := tempDir + "/" + fileName
		fileData, _ := os.ReadFile(filePath)

		up, err := client.Upload(context.Background(), fileData, whatsmeow.MediaDocument)
		if err != nil {
			replyMessage(client, v, "❌ WhatsApp upload failed.")
			return
		}

		
		
		client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			DocumentMessage: &waProto.DocumentMessage{
				URL:           proto.String(up.URL),
				DirectPath:    proto.String(up.DirectPath),
				MediaKey:      up.MediaKey,
				Mimetype:      proto.String("application/octet-stream"),
				Title:         proto.String(fileName),
				FileName:      proto.String(fileName),
				FileLength:    proto.Uint64(uint64(len(fileData))),
				FileSHA256:    up.FileSHA256,
				FileEncSHA256: up.FileEncSHA256,
				ContextInfo: &waProto.ContextInfo{
					ExternalAdReply: &waProto.ContextInfo_ExternalAdReplyInfo{
						Title:     proto.String("Impossible Mega Engine"),
						Body:      proto.String("File: " + fileName),
						SourceURL: proto.String(urlStr),
						
						MediaType: waProto.ContextInfo_ExternalAdReplyInfo_IMAGE.Enum(), 
					},
				},
			},
		})
		
		react(client, v.Info.Chat, v.Info.ID, "✅")
	}()
}


func handleTed(client *whatsmeow.Client, v *events.Message, url string) {
	if url == "" { replyMessage(client, v, "⚠️ Provide a TED link."); return }
	react(client, v.Info.Chat, v.Info.ID, "🎓")
	sendPremiumCard(client, v, "TED Talks", "Knowledge-Hub", "💡 Extracting HD Lesson...")
	go downloadAndSend(client, v, url, "video")
}








var ttCache = make(map[string]TTState)


func sendPremiumCard(client *whatsmeow.Client, v *events.Message, title, site, info string) {
	card := fmt.Sprintf(`╔══════════════════════╗
║ ✨ %s DOWNLOADER
╠══════════════════════╣
║ 📝 Title: %s
║ 🌐 Site: %s
╠══════════════════════╣
║ ⏳ Status: Processing...
╚══════════════════════╝
%s`, strings.ToUpper(site), title, site, info)
	replyMessage(client, v, card)
}




func downloadAndSend(client *whatsmeow.Client, v *events.Message, ytUrl, mode string, optionalFormat ...string) {
	react(client, v.Info.Chat, v.Info.ID, "⏳")
	
	fileName := fmt.Sprintf("temp_%d", time.Now().UnixNano())
	formatArg := "bestvideo[height<=720][ext=mp4]+bestaudio[ext=m4a]/best"
	if len(optionalFormat) > 0 && optionalFormat[0] != "" {
		formatArg = optionalFormat[0]
	}

	var args []string
	if mode == "audio" {
		fileName += ".mp3"
		args = []string{"--no-playlist", "-f", "bestaudio", "--extract-audio", "--audio-format", "mp3", "-o", fileName, ytUrl}
	} else {
		fileName += ".mp4"
		args = []string{"--no-playlist", "-f", formatArg, "--merge-output-format", "mp4", "-o", fileName, ytUrl}
	}

	
	fullCmd := strings.Join(args, " ")

	cmd := exec.Command("yt-dlp", args...)
	output, err := cmd.CombinedOutput() 
	if err != nil {
		replyMessage(client, v, "❌ Media processing failed. Check logs for details.")
		return
	}

	

	
	fileData, err := os.ReadFile(fileName)
	if err != nil { return }
	defer os.Remove(fileName)

	fileSize := uint64(len(fileData))
	mType := whatsmeow.MediaVideo
	if mode == "audio" { mType = whatsmeow.MediaDocument }

	up, err := client.Upload(context.Background(), fileData, mType)
	if err != nil {
		replyMessage(client, v, "❌ Failed to upload to WhatsApp servers.")
		return
	}

	
	var finalMsg waProto.Message
	if mode == "audio" {
		finalMsg.DocumentMessage = &waProto.DocumentMessage{
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			Mimetype: proto.String("audio/mpeg"), FileName: proto.String("Impossible_Audio.mp3"),
			FileLength: proto.Uint64(fileSize), FileSHA256: up.FileSHA256, FileEncSHA256: up.FileEncSHA256,
		}
	} else {
		finalMsg.VideoMessage = &waProto.VideoMessage{
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			Mimetype: proto.String("video/mp4"), Caption: proto.String("✅ *Impossible Bot - Success*"),
			FileLength: proto.Uint64(fileSize), FileSHA256: up.FileSHA256, FileEncSHA256: up.FileEncSHA256,
		}
	}

	client.SendMessage(context.Background(), v.Info.Chat, &finalMsg)
	react(client, v.Info.Chat, v.Info.ID, "✅")
}




func handleFacebook(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Facebook Video", "Facebook", "🎥 Extracting High Quality Content...")
	go downloadAndSend(client, v, url, "video")
}

func handleInstagram(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Instagram Reel", "Instagram", "📸 Capturing Media...")
	go downloadAndSend(client, v, url, "video")
}

func handleTikTok(client *whatsmeow.Client, v *events.Message, urlStr string) {
	if urlStr == "" { return }
	react(client, v.Info.Chat, v.Info.ID, "🎵")
	
	apiUrl := "https://www.tikwm.com/api/?url=" + url.QueryEscape(urlStr)
	var r struct { 
		Code int `json:"code"`
		Data struct { Play, Music, Title string; Size uint64 } `json:"data"` 
	}
	getJson(apiUrl, &r)

	if r.Code == 0 {
		
		sender := v.Info.Sender.ToNonAD().String() 
		ttCache[sender] = TTState{
			PlayURL: r.Data.Play, 
			MusicURL: r.Data.Music, 
			Title: r.Data.Title, 
			Size: int64(r.Data.Size),
		}

		
		menuText := fmt.Sprintf("📝 *Title:* %s\n\n", r.Data.Title)
		menuText += "🔢 *Reply with a number:*\n\n"
		menuText += "  【 1 】 🎬 *Video (No WM)*\n"
		menuText += "  【 2 】 🎵 *Audio (MP3)*\n"
		menuText += "  【 3 】 📄 *Full Info*\n\n"
		menuText += "⏳ *Timeout:* 2 Minutes"

		sendPremiumCard(client, v, "TikTok Downloader", "TikWM Engine", menuText)
	} else {
		replyMessage(client, v, "❌ *Error:* Could not fetch TikTok data.")
	}
}



func sendAudio(client *whatsmeow.Client, v *events.Message, audioURL string) {
	
	resp, err := http.Get(audioURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	
	up, err := client.Upload(context.Background(), data, whatsmeow.MediaAudio)
	if err != nil {
		return
	}

	
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("audio/mpeg"), 
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
			PTT:           proto.Bool(false), 
		},
	})
}


func handleTikTokReply(client *whatsmeow.Client, v *events.Message, input string, senderID string) {
	
	state, exists := ttCache[senderID]
	if !exists { return }

	
	
	senderID = v.Info.Sender.ToNonAD().String() 

	input = strings.TrimSpace(input)

	switch input {
	case "1":
		react(client, v.Info.Chat, v.Info.ID, "🎬")
		sendVideo(client, v, state.PlayURL, "✅ *TikTok Video Generated*")
		delete(ttCache, senderID) 

	case "2":
		react(client, v.Info.Chat, v.Info.ID, "🎵")
		
		sendAudio(client, v, state.MusicURL)  
		delete(ttCache, senderID)

	case "3":
		infoMsg := fmt.Sprintf("╔═══════════════════╗\n"+
			"║      ✨ TIKTOK INFO ✨     ║\n"+
			"╠═══════════════════╣\n"+
			"║ 📝 Title: %s\n"+
			"║ 📊 Size: %.2f MB\n"+
			"╚═══════════════════╝", state.Title, float64(state.Size)/(1024*1024))
		replyMessage(client, v, infoMsg)
		delete(ttCache, senderID)
	}
}

func handleTwitter(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "X Video", "Twitter/X", "🐦 Speeding through X servers...")
	go downloadAndSend(client, v, url, "video")
}

func handlePinterest(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Pin Media", "Pinterest", "📌 Extracting Media Asset...")
	go downloadAndSend(client, v, url, "video")
}

func handleThreads(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Threads Clip", "Threads", "🧵 Processing Thread...")
	go downloadAndSend(client, v, url, "video")
}

func handleSnapchat(client *whatsmeow.Client, v *events.Message, url string) {
	if url == "" { return }
	react(client, v.Info.Chat, v.Info.ID, "👻")
	sendPremiumCard(client, v, "Snapchat", "Snap-Engine", "👻 Capturing Snap Spotlight... Please wait.")
	
	
	go downloadAndSend(client, v, url, "video")
}

func handleReddit(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Reddit Post", "Reddit", "👽 Merging Audio & Video...")
	go downloadAndSend(client, v, url, "video")
}


func handleYoutubeVideo(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "YouTube HD", "YouTube", "🎬 Fetching 720p/1080p Stream...")
	go downloadAndSend(client, v, url, "video")
}

func handleYoutubeAudio(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "YouTube MP3", "YouTube", "🎶 Converting to 320kbps Audio...")
	go downloadAndSend(client, v, url, "audio")
}

func handleTwitch(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Twitch Clip", "Twitch", "🎮 Grabbing Stream Moment...")
	go downloadAndSend(client, v, url, "video")
}

func handleDailyMotion(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "DailyMotion", "DailyMotion", "📺 Packing Video Stream...")
	go downloadAndSend(client, v, url, "video")
}

func handleVimeo(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Vimeo Pro", "Vimeo", "✨ Professional Extraction...")
	go downloadAndSend(client, v, url, "video")
}

func handleRumble(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Rumble Stream", "Rumble", "🥊 Fetching Rumble Media...")
	go downloadAndSend(client, v, url, "video")
}

func handleBilibili(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Anime Video", "Bilibili", "💮 Accessing Bilibili Nodes...")
	go downloadAndSend(client, v, url, "video")
}

func handleBitChute(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Alt Video", "BitChute", "🎞️ Extraction Started...")
	go downloadAndSend(client, v, url, "video")
}


func handleSoundCloud(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Music Track", "SoundCloud", "🎧 Ripping HQ Audio...")
	go downloadAndSend(client, v, url, "audio")
}

func handleSpotify(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Spotify Track", "Spotify", "🎵 Extracting from Spotify...")
	go downloadAndSend(client, v, url, "audio")
}

func handleAppleMusic(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Apple Preview", "AppleMusic", "🎶 Grabbing High-Fi Clip...")
	go downloadAndSend(client, v, url, "audio")
}

func handleDeezer(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Deezer HQ", "Deezer", "🎼 Converting Track...")
	go downloadAndSend(client, v, url, "audio")
}

func handleTidal(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Tidal Master", "Tidal", "💎 Fetching Lossless Audio...")
	go downloadAndSend(client, v, url, "audio")
}

func handleMixcloud(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "DJ Mixset", "Mixcloud", "🎧 Extracting Long Set...")
	go downloadAndSend(client, v, url, "audio")
}

func handleNapster(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Legacy Track", "Napster", "🎶 Downloading Music...")
	go downloadAndSend(client, v, url, "audio")
}

func handleBandcamp(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Indie Music", "Bandcamp", "🎸 Grabbing Artist Track...")
	go downloadAndSend(client, v, url, "audio")
}


func handleImgur(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Imgur Media", "Imgur", "🖼️ Extracting Image/Video...")
	go downloadAndSend(client, v, url, "video")
}

func handleGiphy(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Animated GIF", "Giphy", "🎞️ Rendering GIF Stream...")
	go downloadAndSend(client, v, url, "video")
}

func handleFlickr(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "HQ Assets", "Flickr", "📸 Fetching Media...")
	go downloadAndSend(client, v, url, "video")
}

func handle9Gag(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Meme Video", "9Gag", "🤣 Grabbing Viral Content...")
	go downloadAndSend(client, v, url, "video")
}

func handleIfunny(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Funny Media", "iFunny", "🤡 Processing Meme...")
	go downloadAndSend(client, v, url, "video")
}


func handleGithub(client *whatsmeow.Client, v *events.Message, urlStr string) {
	if urlStr == "" { return }
	
	
	urlStr = strings.TrimSuffix(urlStr, ".git")
	urlStr = strings.TrimSuffix(urlStr, "/")
	
	react(client, v.Info.Chat, v.Info.ID, "💻")
	sendPremiumCard(client, v, "Repo Source", "GitHub", "📁 Packing Repository ZIP...")

	zipURL := urlStr + "/zipball/HEAD"

	
	resp, err := http.Get(zipURL)
	if err != nil || resp.StatusCode != 200 {
		replyMessage(client, v, "❌ *GitHub Error:* Repo not found. Ensure it is public.")
		return
	}
	defer resp.Body.Close()

	fileName := fmt.Sprintf("repo_%d.zip", time.Now().UnixNano())
	out, _ := os.Create(fileName)
	io.Copy(out, resp.Body)
	out.Close()

	fileData, _ := os.ReadFile(fileName)
	defer os.Remove(fileName)

	up, err := client.Upload(context.Background(), fileData, whatsmeow.MediaDocument)
	if err != nil { return }

	
		client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			DocumentMessage: &waProto.DocumentMessage{
				URL:           proto.String(up.URL),
				DirectPath:    proto.String(up.DirectPath),
				MediaKey:      up.MediaKey,
				Mimetype:      proto.String("application/octet-stream"),
				Title:         proto.String(fileName),
				FileName:      proto.String(fileName),
				FileLength:    proto.Uint64(uint64(len(fileData))),
				FileSHA256:    up.FileSHA256,
				FileEncSHA256: up.FileEncSHA256,
				ContextInfo: &waProto.ContextInfo{
					ExternalAdReply: &waProto.ContextInfo_ExternalAdReplyInfo{
						Title:     proto.String("Impossible Mega Engine"),
						Body:      proto.String("File: " + fileName),
						SourceURL: proto.String(urlStr),
						MediaType: waProto.ContextInfo_ExternalAdReplyInfo_IMAGE.Enum(), 
					},
				},
			},
		})
	react(client, v.Info.Chat, v.Info.ID, "✅")
}

func handleArchive(client *whatsmeow.Client, v *events.Message, urlStr string) {
	if urlStr == "" { return }
	
	urlStr = strings.TrimSpace(urlStr)
	react(client, v.Info.Chat, v.Info.ID, "🏛️")
	sendPremiumCard(client, v, "Archive Downloader", "Wayback-Machine", "🏛️ Accessing historical servers...")

	go func() {
		
		clientHttp := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil 
			},
		}

		req, _ := http.NewRequest("GET", urlStr, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
		
		resp, err := clientHttp.Do(req)
		if err != nil || resp.StatusCode != 200 {
			replyMessage(client, v, "❌ *Archive Error:* Could not reach the file. Link might be dead.")
			return
		}
		defer resp.Body.Close()

		
		fileName := "archive_file"
		if disp := resp.Header.Get("Content-Disposition"); strings.Contains(disp, "filename=") {
			fileName = strings.Split(disp, "filename=")[1]
			fileName = strings.Trim(fileName, ` "`)
		} else {
			
			parts := strings.Split(urlStr, "/")
			fileName = parts[len(parts)-1]
			if !strings.Contains(fileName, ".") { fileName += ".bin" }
		}

		
		tempFile := fmt.Sprintf("temp_arc_%d_%s", time.Now().UnixNano(), fileName)
		out, _ := os.Create(tempFile)
		_, err = io.Copy(out, resp.Body)
		out.Close()

		if err != nil {
			replyMessage(client, v, "❌ *Error:* Download interrupted.")
			os.Remove(tempFile)
			return
		}

		fileData, _ := os.ReadFile(tempFile)
		defer os.Remove(tempFile)

		
		
		up, err := client.Upload(context.Background(), fileData, whatsmeow.MediaDocument)
		if err != nil {
			replyMessage(client, v, "❌ WhatsApp upload failed.")
			return
		}

		
		client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			DocumentMessage: &waProto.DocumentMessage{
				URL:           proto.String(up.URL),
				DirectPath:    proto.String(up.DirectPath),
				MediaKey:      up.MediaKey,
				Mimetype:      proto.String(resp.Header.Get("Content-Type")),
				Title:         proto.String(fileName),
				FileName:      proto.String(fileName),
				FileLength:    proto.Uint64(uint64(len(fileData))),
				FileSHA256:    up.FileSHA256,
				FileEncSHA256: up.FileEncSHA256,
				ContextInfo: &waProto.ContextInfo{
					ExternalAdReply: &waProto.ContextInfo_ExternalAdReplyInfo{
						Title:     proto.String("Impossible Archive Engine"),
						Body:      proto.String("Restored from Wayback Machine"),
						SourceURL: proto.String(urlStr),
						
						MediaType: waProto.ContextInfo_ExternalAdReplyInfo_IMAGE.Enum(),
					},
				},
			},
		})
		
		react(client, v.Info.Chat, v.Info.ID, "✅")
	}()
}


func handleYTS(client *whatsmeow.Client, v *events.Message, query string) {
	if query == "" { return }
	react(client, v.Info.Chat, v.Info.ID, "🔍")
	
	
	myID := getCleanID(client.Store.ID.User)

	cmd := exec.Command("yt-dlp", "ytsearch5:"+query, "--get-title", "--get-id", "--no-playlist")
	out, _ := cmd.Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 { return }

	var results []YTSResult
	
	menuText := "╭─── 📺 *YOUTUBE SEARCH* ───╮\n│\n"
	
	for i := 0; i < len(lines)-1; i += 2 {
		title := lines[i]
		results = append(results, YTSResult{Title: title, Url: "https://www.youtube.com/watch?v=" + lines[i+1]})
		menuText += fmt.Sprintf("📍 *[%d]* %s\n│ ┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈\n", (i/2)+1, title)
	}
	menuText += "│\n╰────────────────────╯"

	resp, err := client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{Text: proto.String(menuText)},
	})

	if err == nil {
		ytCache[resp.ID] = YTSession{Results: results, SenderID: v.Info.Sender.User, BotLID: myID}
		go func() { time.Sleep(2 * time.Minute); delete(ytCache, resp.ID) }()
	}
}

func handleYTDownloadMenu(client *whatsmeow.Client, v *events.Message, ytUrl string) {
	myID := getCleanID(client.Store.ID.User)
	senderLID := v.Info.Sender.User

	menu := `╔════════════════════╗
║    🎬 VIDEO SELECTOR 
╠════════════════════╣
║ 1️⃣ 360p (Fast)
║ 2️⃣ 720p (HD)
║ 3️⃣ 1080p (FHD)
║ 4️⃣ MP3 (Audio)
║
║ ⏳ Select an option by 
║ replying to this card.
╚════════════════════╝`

	resp, err := client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{Text: proto.String(menu)},
	})

	if err == nil {
		
		ytDownloadCache[resp.ID] = YTState{
			Url:      ytUrl,
			BotLID:   myID,
			SenderID: senderLID,
		}
		
		
		go func() {
			time.Sleep(1 * time.Minute)
			delete(ytDownloadCache, resp.ID)
		}()
	}
}

func handleYTDownload(client *whatsmeow.Client, v *events.Message, ytUrl, choice string, isAudio bool) {
	react(client, v.Info.Chat, v.Info.ID, "⏳")
	
	mode := "video"
	format := "bestvideo[height<=720]+bestaudio/best" 

	if isAudio {
		mode = "audio"
	} else {
		switch choice {
		case "1": format = "bestvideo[height<=360]+bestaudio/best"
		case "2": format = "bestvideo[height<=720]+bestaudio/best"
		case "3": format = "bestvideo[height<=1080]+bestaudio/best"
		}
	}

	
	go downloadAndSend(client, v, ytUrl, mode, format) 
}



func getJson(url string, target interface{}) error {
	r, err := http.Get(url); if err != nil { return err }; defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func sendVideo(client *whatsmeow.Client, v *events.Message, videoURL, caption string) {
	go downloadAndSend(client, v, videoURL, "video")
}

func sendDocument(client *whatsmeow.Client, v *events.Message, docURL, name, mime string) {
	resp, err := http.Get(docURL); if err != nil { return }; defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	up, _ := client.Upload(context.Background(), data, whatsmeow.MediaDocument)
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		DocumentMessage: &waProto.DocumentMessage{
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			Mimetype: proto.String(mime), FileName: proto.String(name), FileLength: proto.Uint64(uint64(len(data))),
		},
	})
}





func handler(botClient *whatsmeow.Client, evt interface{}) {
	
	defer func() {
		if r := recover(); r != nil {
		}
	}()

	if botClient == nil {
		return
	}
	
	switch v := evt.(type) {
	case *events.Message:
		
		go processMessage(botClient, v)
	case *events.GroupInfo:
		go handleGroupInfoChange(botClient, v)
	case *events.Connected, *events.LoggedOut:
		
	}
}

func isKnownCommand(text string) bool {
	commands := []string{
		"menu", "help", "list", "ping", "id", "owner", "data", "listbots",
		"alwaysonline", "autoread", "autoreact", "autostatus", "statusreact",
		"addstatus", "delstatus", "liststatus", "readallstatus", "setprefix", "mode",
		"antilink", "antipic", "antivideo", "antisticker",
		"kick", "add", "promote", "demote", "tagall", "hidetag", "group", "del", "delete",
		"tiktok", "tt", "fb", "facebook", "insta", "ig", "pin", "pinterest", "ytmp3", "ytmp4",
		"sticker", "s", "toimg", "tovideo", "removebg", "remini", "tourl", "weather", "translate", "tr", "vv",
	}

	lowerText := strings.ToLower(strings.TrimSpace(text))
	for _, cmd := range commands {
		if strings.HasPrefix(lowerText, cmd) {
			return true
		}
	}
	return false
}



func processMessage(client *whatsmeow.Client, v *events.Message) {
	
	rawBotID := client.Store.ID.User
	botID := botCleanIDCache[rawBotID]
	if botID == "" { botID = getCleanID(rawBotID) } 
	
	prefix := getPrefix(botID)
	bodyRaw := getText(v.Message)
	if bodyRaw == "" { return }
	
	bodyClean := strings.TrimSpace(bodyRaw)
	
	senderID := v.Info.Sender.ToNonAD().String() 
	chatID := v.Info.Chat.String()
	isGroup := v.Info.IsGroup

	
	var qID string
	if extMsg := v.Message.GetExtendedTextMessage(); extMsg != nil && extMsg.ContextInfo != nil {
		qID = extMsg.ContextInfo.GetStanzaID()
	}

	
	session, isYTS := ytCache[qID]
	stateYT, isYTSelect := ytDownloadCache[qID]
	_, isSetup := setupMap[qID]
	_, isTT := ttCache[senderID] 

	
	if isGroup {
    // پہلے سیٹنگز چیک کریں، پھر Goroutine چلائیں
        s := getGroupSettings(chatID)
        if s.Antilink || s.AntiPic || s.AntiVideo || s.AntiSticker {
            go checkSecurity(client, v)
        }
    }


	
	isAnySession := isSetup || isYTS || isYTSelect || isTT
	if !strings.HasPrefix(bodyClean, prefix) && !isAnySession && chatID != "status@broadcast" {
		return 
	}

	

	
	if isSetup {
		handleSetupResponse(client, v)
		return
	}

	
	if isTT && !strings.HasPrefix(bodyClean, prefix) {
		if bodyClean == "1" || bodyClean == "2" || bodyClean == "3" {
			handleTikTokReply(client, v, bodyClean, senderID)
			return
		}
	}

	
	if qID != "" {
		
		if isYTS && session.BotLID == botID {
			var idx int
			n, _ := fmt.Sscanf(bodyClean, "%d", &idx)
			if n > 0 && idx >= 1 && idx <= len(session.Results) {
				delete(ytCache, qID)
				handleYTDownloadMenu(client, v, session.Results[idx-1].Url)
				return
			}
		}
		
		if isYTSelect && stateYT.BotLID == botID {
			delete(ytDownloadCache, qID)
			go handleYTDownload(client, v, stateYT.Url, bodyClean, (bodyClean == "4"))
			return
		}
	}

	
	if chatID == "status@broadcast" {
		dataMutex.RLock()
		if data.AutoStatus {
			client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender)
			if data.StatusReact {
				emojis := []string{"💚", "❤️", "🔥", "😍", "💯"}
				react(client, v.Info.Chat, v.Info.ID, emojis[time.Now().UnixNano()%int64(len(emojis))])
			}
		}
		dataMutex.RUnlock()
		return
	}

	
	dataMutex.RLock()
	if data.AutoRead { client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender) }
	if data.AutoReact { react(client, v.Info.Chat, v.Info.ID, "❤️") }
	dataMutex.RUnlock()

	
	msgWithoutPrefix := strings.TrimPrefix(bodyClean, prefix)
	words := strings.Fields(msgWithoutPrefix)
	
	if len(words) == 0 { return }
	
	
	cmd := strings.ToLower(words[0]) 
	
	
	fullArgs := strings.TrimSpace(strings.Join(words[1:], " "))

	if !canExecute(client, v, cmd) { return }


	switch cmd {
	case "setprefix":
		if !isOwner(client, v.Info.Sender) { replyMessage(client, v, "❌ Only Owner can change the prefix."); return }
		if fullArgs == "" { replyMessage(client, v, "⚠️ Usage: .setprefix !"); return }
		updatePrefixDB(botID, fullArgs)
		replyMessage(client, v, fmt.Sprintf("✅ Prefix updated to [%s]", fullArgs))

	case "menu", "help", "list":
		react(client, v.Info.Chat, v.Info.ID, "📜"); sendMenu(client, v)
	case "ping":
		react(client, v.Info.Chat, v.Info.ID, "⚡"); sendPing(client, v)
	case "id":
		sendID(client, v)
	case "owner":
		sendOwner(client, v)
	case "listbots":
		sendBotsList(client, v)
	case "data":
		replyMessage(client, v, "╔════════════════╗\n║ 📂 DATA STATUS\n╠════════════════╣\n║ ✅ System Active\n╚════════════════╝")
	case "alwaysonline":
		toggleAlwaysOnline(client, v)
	case "autoread":
		toggleAutoRead(client, v)
	case "autoreact":
		toggleAutoReact(client, v)
	case "autostatus":
		toggleAutoStatus(client, v)
	case "statusreact":
		toggleStatusReact(client, v)
	case "addstatus":
		handleAddStatus(client, v, words[1:])
	case "delstatus":
		handleDelStatus(client, v, words[1:])
	case "liststatus":
		handleListStatus(client, v)
	case "readallstatus":
		handleReadAllStatus(client, v)
	case "mode":
		handleMode(client, v, words[1:])
	case "antilink":
		startSecuritySetup(client, v, "antilink")
	case "antipic":
		startSecuritySetup(client, v, "antipic")
	case "antivideo":
		startSecuritySetup(client, v, "antivideo")
	case "antisticker":
		startSecuritySetup(client, v, "antisticker")
	case "kick":
		handleKick(client, v, words[1:])
	case "add":
		handleAdd(client, v, words[1:])
	case "promote":
		handlePromote(client, v, words[1:])
	case "demote":
		handleDemote(client, v, words[1:])
	case "tagall":
		handleTagAll(client, v, words[1:])
	case "hidetag":
		handleHideTag(client, v, words[1:])
	case "group":
		handleGroup(client, v, words[1:])
	case "del", "delete":
		handleDelete(client, v)
	case "toimg": 
	    handleToImg(client, v)
    
    case "tovideo":
        handleToMedia(client, v, false) 

    case "togif":
        handleToMedia(client, v, true)  
    case "s", "sticker": 
        handleToSticker(client, v)
	case "tourl":
		handleToURL(client, v)
	case "translate", "tr":
		handleTranslate(client, v, words[1:])
	case "vv":
		handleVV(client, v)
	case "sd":
		handleSessionDelete(client, v, words[1:])
	case "yts":
		handleYTS(client, v, fullArgs)

	
	case "yt", "ytmp4", "ytmp3", "ytv", "yta", "youtube":
		if fullArgs == "" {
			replyMessage(client, v, "⚠️ *Usage:* .yt [YouTube Link]")
			return
		}
		
		if strings.Contains(strings.ToLower(fullArgs), "youtu") {
			handleYTDownloadMenu(client, v, fullArgs) 
		} else {
			replyMessage(client, v, "❌ Please provide a valid YouTube link.")
		}

	case "fb", "facebook":
		handleFacebook(client, v, fullArgs)
	case "ig", "insta", "instagram":
		handleInstagram(client, v, fullArgs)
	case "tt", "tiktok":
		handleTikTok(client, v, fullArgs)
	case "tw", "x", "twitter":
		handleTwitter(client, v, fullArgs)
	case "pin", "pinterest":
		handlePinterest(client, v, fullArgs)
	case "threads":
		handleThreads(client, v, fullArgs)
	case "snap", "snapchat":
		handleSnapchat(client, v, fullArgs)
	case "reddit":
		handleReddit(client, v, fullArgs)
	case "twitch":
		handleTwitch(client, v, fullArgs)
	case "dm", "dailymotion":
		handleDailyMotion(client, v, fullArgs)
	case "vimeo":
		handleVimeo(client, v, fullArgs)
	case "rumble":
		handleRumble(client, v, fullArgs)
	case "bilibili":
		handleBilibili(client, v, fullArgs)
	case "douyin":
		handleDouyin(client, v, fullArgs)
	case "kwai":
		handleKwai(client, v, fullArgs)
	case "bitchute":
		handleBitChute(client, v, fullArgs)
	case "sc", "soundcloud":
		handleSoundCloud(client, v, fullArgs)
	case "spotify":
		handleSpotify(client, v, fullArgs)
	case "apple", "applemusic":
		handleAppleMusic(client, v, fullArgs)
	case "deezer":
		handleDeezer(client, v, fullArgs)
	case "tidal":
		handleTidal(client, v, fullArgs)
	case "mixcloud":
		handleMixcloud(client, v, fullArgs)
	case "napster":
		handleNapster(client, v, fullArgs)
	case "bandcamp":
		handleBandcamp(client, v, fullArgs)
	case "imgur":
		handleImgur(client, v, fullArgs)
	case "giphy":
		handleGiphy(client, v, fullArgs)
	case "flickr":
		handleFlickr(client, v, fullArgs)
	case "9gag":
		handle9Gag(client, v, fullArgs)
	case "ifunny":
		handleIfunny(client, v, fullArgs)
	case "stats", "server", "dashboard":
		handleServerStats(client, v)
	case "speed", "speedtest":
		handleSpeedTest(client, v)
	case "ss", "screenshot":
		handleScreenshot(client, v, fullArgs)
    case "ai", "ask", "gpt":
        handleAI(client, v, fullArgs, cmd) 
	case "imagine", "img", "draw":
		handleImagine(client, v, fullArgs)
	case "google", "search":
		handleGoogle(client, v, fullArgs)
	case "weather":
		handleWeather(client, v, fullArgs)
	case "remini", "upscale", "hd":
		handleRemini(client, v)
	case "removebg", "rbg":
		handleRemoveBG(client, v)
	case "fancy", "style":
		handleFancy(client, v, fullArgs)
	case "toptt", "voice":
		handleToPTT(client, v)
	case "ted":
		handleTed(client, v, fullArgs)
	case "steam":
		handleSteam(client, v, fullArgs)
	case "archive":
		handleArchive(client, v, fullArgs)
	case "git", "github":
		handleGithub(client, v, fullArgs)
	case "dl", "download", "mega":
		handleMega(client, v, fullArgs)
	}
}



func getPrefix(botID string) string {
	prefixMutex.RLock()
	p, exists := botPrefixes[botID]
	prefixMutex.RUnlock()
	if exists {
		return p
	}
	
	val, err := rdb.Get(context.Background(), "prefix:"+botID).Result()
	if err != nil || val == "" {
		return "." 
	}
	prefixMutex.Lock()
	botPrefixes[botID] = val
	prefixMutex.Unlock()
	return val
}

func getCleanID(jidStr string) string {
	if jidStr == "" { return "unknown" }
	parts := strings.Split(jidStr, "@")
	if len(parts) == 0 { return "unknown" }
	userPart := parts[0]
	if strings.Contains(userPart, ":") {
		userPart = strings.Split(userPart, ":")[0]
	}
	if strings.Contains(userPart, ".") {
		userPart = strings.Split(userPart, ".")[0]
	}
	return strings.TrimSpace(userPart)
}


func getBotLIDFromDB(client *whatsmeow.Client) string {
	
	if client.Store.LID.IsEmpty() { 
		return "unknown" 
	}
	
	return getCleanID(client.Store.LID.User)
}


func isOwner(client *whatsmeow.Client, sender types.JID) bool {
	
	if client.Store.LID.IsEmpty() { 
		return false 
	}

	
	senderLID := getCleanID(sender.User)

	
	botLID := getCleanID(client.Store.LID.User)

	
	
	return senderLID == botLID
}

func isAdmin(client *whatsmeow.Client, chat, user types.JID) bool {
    chatID := chat.String()
    userNum := getCleanID(user.User)

    // 1️⃣ پہلے کیش (RAM) چیک کریں
    adminCacheMutex.RLock()
    cached, exists := adminCache[chatID]
    adminCacheMutex.RUnlock()

    // اگر ڈیٹا موجود ہے اور 5 منٹ سے زیادہ پرانا نہیں ہے، تو وہیں سے جواب دیں
    if exists && time.Since(cached.Timestamp) < 5*time.Minute {
        return cached.Admins[userNum]
    }

    // 2️⃣ اگر کیش میں نہیں ہے، تو واٹس ایپ سے فریش ڈیٹا منگوائیں (Network Call)
    info, err := client.GetGroupInfo(context.Background(), chat)
    if err != nil {
        return false
    }

    // 3️⃣ نئی لسٹ تیار کریں
    newAdmins := make(map[string]bool)
    for _, p := range info.Participants {
        if p.IsAdmin || p.IsSuperAdmin {
            cleanP := getCleanID(p.JID.User)
            newAdmins[cleanP] = true
        }
    }

    // 4️⃣ کیش اپڈیٹ کریں
    adminCacheMutex.Lock()
    adminCache[chatID] = CachedAdminList{
        Admins:    newAdmins,
        Timestamp: time.Now(),
    }
    adminCacheMutex.Unlock()

    // 5️⃣ رزلٹ واپس کریں
    return newAdmins[userNum]
}


func canExecute(client *whatsmeow.Client, v *events.Message, cmd string) bool {
	if isOwner(client, v.Info.Sender) { return true }
	if !v.Info.IsGroup { return true }
	s := getGroupSettings(v.Info.Chat.String())
	if s.Mode == "private" { return false }
	if s.Mode == "admin" { return isAdmin(client, v.Info.Chat, v.Info.Sender) }
	return true
}

func sendOwner(client *whatsmeow.Client, v *events.Message) {
	
	isMatch := isOwner(client, v.Info.Sender)
	
	
	
	botLID := getBotLIDFromDB(client)
	
	
	senderLID := getCleanID(v.Info.Sender.User)
	
	
	status := "❌ NOT Owner"
	emoji := "🚫"
	if isMatch {
		status = "✅ YOU are Owner"
		emoji = "👑"
	}
	
	
	
	
	
	msg := fmt.Sprintf(`╔═══════════════════╗
║ %s OWNER VERIFICATION
╠═══════════════════╣
║ 🆔 Bot LID  : %s
║ 👤 Your LID : %s
╠═══════════════════╣
║ 📊 Status: %s
╚═══════════════════╝`, emoji, botLID, senderLID, status)
	
	replyMessage(client, v, msg)
}

func sendBotsList(client *whatsmeow.Client, v *events.Message) {
	clientsMutex.RLock()
	count := len(activeClients)
	msg := fmt.Sprintf(`╔═══════════════════╗
║ 📊 MULTI-BOT STATUS
╠═══════════════════╣
║ 🤖 Active Bots: %d
╠═══════════════════╣`, count)
	i := 1
	for num := range activeClients {
		msg += fmt.Sprintf("\n║ %d. %s", i, num)
		i++
	}
	clientsMutex.RUnlock()
	msg += "\n╚═══════════════════╝"
	replyMessage(client, v, msg)
}

func getFormattedUptime() string {
	seconds := persistentUptime
	days := seconds / 86400
	seconds %= 86400
	hours := seconds / 3600
	seconds %= 3600
	minutes := seconds / 60
	return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
}

func sendMenu(client *whatsmeow.Client, v *events.Message) {
	uptimeStr := getFormattedUptime()
	rawBotID := client.Store.ID.User
	botID := botCleanIDCache[rawBotID]
	p := getPrefix(botID)
	s := getGroupSettings(v.Info.Chat.String())
	currentMode := strings.ToUpper(s.Mode)
	if !strings.Contains(v.Info.Chat.String(), "@g.us") { currentMode = "PRIVATE" }

	menu := fmt.Sprintf(`╔══════════════════════╗
║     ✨ %s ✨     
╠══════════════════════╣
║ 👋 *Assalam-o-Alaikum*
║ 👑 *Owner:* %s              
║ 🛡️ *Mode:* %s               
║ ⏳ *Uptime:* %s             
╠══════════════════════╣
║                           
║ ╭─── SOCIAL DOWNLOADERS ──╮
║ │ 🔸 *%sfb* - Facebook Video
║ │ 🔸 *%sig* - Instagram Reel/Post
║ │ 🔸 *%stt* - TikTok No Watermark
║ │ 🔸 *%stw* - Twitter/X Media
║ │ 🔸 *%spin* - Pinterest Downloader
║ │ 🔸 *%sthreads* - Threads Video
║ │ 🔸 *%ssnap* - Snapchat Content
║ │ 🔸 *%sreddit* - Reddit with Audio
║ ╰───────────────────────╯
║                             
║ ╭─── VIDEO & STREAMS ────╮
║ │ 🔸 *%syt* - <Link>
║ │ 🔸 *%syts* - YouTube Search
║ │ 🔸 *%stwitch* - Twitch Clips
║ │ 🔸 *%sdm* - DailyMotion HQ
║ │ 🔸 *%svimeo* - Vimeo Pro Video
║ │ 🔸 *%srumble* - Rumble Stream
║ │ 🔸 *%sbilibili* - Bilibili Anime
║ │ 🔸 *%sdouyin* - Chinese TikTok
║ │ 🔸 *%skwai* - Kwai Short Video
║ │ 🔸 *%sbitchute* - BitChute Alt
║ ╰───────────────────────╯
║
║ ╭─── MUSIC PLATFORMS ────╮
║ │ 🔸 *%ssc* - SoundCloud Music
║ │ 🔸 *%sspotify* - Spotify Track
║ │ 🔸 *%sapple* - Apple Music
║ │ 🔸 *%sdeezer* - Deezer Rippin
║ │ 🔸 *%stidal* - Tidal HQ Audio
║ │ 🔸 *%smixcloud* - DJ Mixsets
║ │ 🔸 *%snapster* - Napster Legacy
║ │ 🔸 *%sbandcamp* - Indie Music
║ ╰───────────────────────╯
║                             
║ ╭────── GROUP ADMIN ──────╮
║ │ 🔸 *%sadd* - Add New Member
║ │ 🔸 *%sdemote* - Remove Admin
║ │ 🔸 *%sgroup* - Group Settings
║ │ 🔸 *%shidetag* - Hidden Mention
║ │ 🔸 *%skick* - Remove Member    
║ │ 🔸 *%spromote* - Make Admin
║ │ 🔸 *%stagall* - Mention Everyone
║ ╰───────────────────────╯
║                             
║ ╭──── BOT SETTINGS ─────╮
║ │ 🔸 *%saddstatus* - Auto Status
║ │ 🔸 *%salwaysonline* - Online 24/7
║ │ 🔸 *%santilink* - Link Protection
║ │ 🔸 *%santipic* - No Images Mode
║ │ 🔸 *%santisticker* - No Stickers
║ │ 🔸 *%santivideo* - No Video Mode
║ │ 🔸 *%sautoreact* - Automatic React
║ │ 🔸 *%sautoread* - Blue Tick Mark
║ │ 🔸 *%sautostatus* - Status View
║ │ 🔸 *%sdelstatus* - Remove Status
║ │ 🔸 *%smode* - Private/Public
║ │ 🔸 *%sstatusreact* - React Status
║ ╰────────────────────────╯
║                             
║ ╭────── AI & TOOLS ─────────╮
║ │ 🔸 *%sstats* - Server Dashboard
║ │ 🔸 *%sspeed* - Internet Speed
║ │ 🔸 *%sss* - Web Screenshot
║ │ 🔸 *%sai* - Artificial Intelligence
║ │ 🔸 *%sask* - Ask Questions
║ │ 🔸 *%sgpt* - GPT 4o Model
║ │ 🔸 *%simg* - Image Generator 
║ │ 🔸 *%sgoogle* - Fast Search
║ │ 🔸 *%sweather* - Climate Info
║ │ 🔸 *%sremini* - HD Image Upscaler
║ │ 🔸 *%sremovebg* - Background Eraser
║ │ 🔸 *%sfancy* - Stylish Text
║ │ 🔸 *%stoptt* - Convert to Audio
║ │ 🔸 *%svv* - ViewOnce Bypass
║ │ 🔸 *%ssticker* - Image to Sticker
║ │ 🔸 *%stoimg* - Sticker to Image
║ │ 🔸 *%stogif* - Sticker To Gif
║ │ 🔸 *%stovideo* - Sticker to Video
║ │ 🔸 *%sgit* - GitHub Downloader
║ │ 🔸 *%sarchive* - Internet Archive
║ │ 🔸 *%smega* - Universal Downloader
║ ╰────────────────────────╯
║                           
╠══════════════════════╣
║ © 2025 Nothing is Impossible 
╚══════════════════════╝`,
		BOT_NAME, OWNER_NAME, currentMode, uptimeStr,
		
		p, p, p, p, p, p, p, p,
		
		p, p, p, p, p, p, p, p, p, p,
		
		p, p, p, p, p, p, p, p,
		
		p, p, p, p, p, p, p,
		
		p, p, p, p, p, p, p, p, p, p, p, p,
		
		p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p)

	sendReplyMessage(client, v, menu)
}

func sendPing(client *whatsmeow.Client, v *events.Message) {
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	ms := time.Since(start).Milliseconds()
	uptimeStr := getFormattedUptime()
	msg := fmt.Sprintf(`╔════════════════╗
║ ⚡ PING STATUS
╠════════════════╣
║ 🚀 Speed: %d MS
║ ⏱️ Uptime: %s
║ 👑 Dev: %s
╠═════════════════════╣
║      🟢 System Running
╚═════════════════════╝`, ms, uptimeStr, OWNER_NAME)
	sendReplyMessage(client, v, msg)
}

func sendID(client *whatsmeow.Client, v *events.Message) {
	user := v.Info.Sender.User
	chat := v.Info.Chat.User
	chatType := "Private"
	if v.Info.IsGroup { chatType = "Group" }
	msg := fmt.Sprintf(`╔════════════════╗
║ 🆔 ID INFO
╠════════════════╣
║ 👤 User ID:
║ `+"`%s`"+`
║ 👥 Chat ID:
║ `+"`%s`"+`
║ 🏷️ Type: %s
╚════════════════╝`, user, chat, chatType)
	sendReplyMessage(client, v, msg)
}

func react(client *whatsmeow.Client, chat types.JID, msgID types.MessageID, emoji string) {
	client.SendMessage(context.Background(), chat, &waProto.Message{
		ReactionMessage: &waProto.ReactionMessage{
			Key: &waProto.MessageKey{
				RemoteJID: proto.String(chat.String()),
				ID:         proto.String(string(msgID)),
				FromMe:     proto.Bool(false),
			},
			Text:              proto.String(emoji),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
		},
	})
}

func replyMessage(client *whatsmeow.Client, v *events.Message, text string) {
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		},
	})
}

func sendReplyMessage(client *whatsmeow.Client, v *events.Message, text string) {
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		},
	})
}

func getText(m *waProto.Message) string {
	if m.Conversation != nil { return *m.Conversation }
	if m.ExtendedTextMessage != nil && m.ExtendedTextMessage.Text != nil { return *m.ExtendedTextMessage.Text }
	if m.ImageMessage != nil && m.ImageMessage.Caption != nil { return *m.ImageMessage.Caption }
	if m.VideoMessage != nil && m.VideoMessage.Caption != nil { return *m.VideoMessage.Caption }
	return ""
}

func getGroupSettings(id string) *GroupSettings {
	cacheMutex.RLock()
	if s, ok := groupCache[id]; ok {
		cacheMutex.RUnlock()
		return s
	}
	cacheMutex.RUnlock()

	s := &GroupSettings{
		ChatID:         id,
		Mode:           "public",
		Antilink:       false,
		AntilinkAdmin:  true,
		AntilinkAction: "delete",
		Warnings:       make(map[string]int),
	}

	cacheMutex.Lock()
	groupCache[id] = s
	cacheMutex.Unlock()
	return s
}

func handleSessionDelete(client *whatsmeow.Client, v *events.Message, args []string) {
	if !isOwner(client, v.Info.Sender) {
		replyMessage(client, v, "╔═══════════════════╗\n║ 👑 OWNER ONLY      \n╠═══════════════════╣\n║ You don't have    \n║ permission.       \n╚═══════════════════╝")
		return
	}
	if len(args) == 0 {
		replyMessage(client, v, "⚠️ Please provide a number.")
		return
	}
	targetNumber := args[0]
	targetJID, ok := parseJID(targetNumber)
	if !ok {
		replyMessage(client, v, "❌ Invalid format.")
		return
	}
	clientsMutex.Lock()
	if targetClient, exists := activeClients[getCleanID(targetNumber)]; exists {
		targetClient.Disconnect()
		delete(activeClients, getCleanID(targetNumber))
	}
	clientsMutex.Unlock()

	if dbContainer == nil {
		replyMessage(client, v, "❌ Database error.")
		return
	}
	device, err := dbContainer.GetDevice(context.Background(), targetJID)
	if err != nil || device == nil {
		replyMessage(client, v, "❌ Not found.")
		return
	}
	device.Delete(context.Background())
	msg := fmt.Sprintf("╔═══════════════════╗\n║ 🗑️ SESSION DELETED  \n╠═══════════════════╣\n║ Number: %s\n╚═══════════════════╝", targetNumber)
	replyMessage(client, v, msg)
}

func parseJID(arg string) (types.JID, bool) {
	if arg == "" { return types.EmptyJID, false }
	if !strings.Contains(arg, "@") { arg += "@s.whatsapp.net" }
	jid, err := types.ParseJID(arg)
	if err != nil { return types.EmptyJID, false }
	return jid, true
}


func handleKick(client *whatsmeow.Client, v *events.Message, args []string) {
	groupAction(client, v, args, "remove")
}

func handleAdd(client *whatsmeow.Client, v *events.Message, args []string) {
	if !v.Info.IsGroup {
		msg := `╔════════════════╗
║ ❌ GROUP ONLY
╠════════════════
║ This command
║ works only in
║ group chats
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	if !isAdmin(client, v.Info.Chat, v.Info.Sender) && !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ DENIED
╠════════════════
║ 🔒 Admin Only
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	if len(args) == 0 {
		msg := `╔════════════════╗
║ ⚠️ INVALID
╠════════════════
║ Usage:
║ .add <number>
║
║ Example:
║ .add 92300xxx
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	num := strings.ReplaceAll(args[0], "+", "")
	jid, _ := types.ParseJID(num + "@s.whatsapp.net")
	client.UpdateGroupParticipants(context.Background(), v.Info.Chat, []types.JID{jid}, whatsmeow.ParticipantChangeAdd)

	msg := fmt.Sprintf(`╔════════════════╗
║ ✅ ADDED
╠════════════════
║ Number: %s
║ Added to group
╚════════════════`, args[0])

	replyMessage(client, v, msg)
}

func handlePromote(client *whatsmeow.Client, v *events.Message, args []string) {
	groupAction(client, v, args, "promote")
}

func handleDemote(client *whatsmeow.Client, v *events.Message, args []string) {
	groupAction(client, v, args, "demote")
}

func handleTagAll(client *whatsmeow.Client, v *events.Message, args []string) {
	if !v.Info.IsGroup {
		msg := `╔════════════════╗
║ ❌ GROUP ONLY
╠════════════════
║ This command
║ works only in
║ group chats
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	if !isAdmin(client, v.Info.Chat, v.Info.Sender) && !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ DENIED
╠════════════════
║ 🔒 Admin Only
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	info, _ := client.GetGroupInfo(context.Background(), v.Info.Chat)
	mentions := []string{}
	out := "╔════════════════╗\n"
	out += "║ 📣 TAG ALL\n"
	out += "╠════════════════\n"

	if len(args) > 0 {
		out += "║ 💬 " + strings.Join(args, " ") + "\n"
	}

	for _, p := range info.Participants {
		mentions = append(mentions, p.JID.String())
		out += "║ @" + p.JID.User + "\n"
	}

	out += fmt.Sprintf("║ 👥 Total: %d\n", len(info.Participants))
	out += "╚════════════════"

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(out),
			ContextInfo: &waProto.ContextInfo{
				MentionedJID: mentions,
				StanzaID:     proto.String(v.Info.ID),
				Participant:  proto.String(v.Info.Sender.String()),
			},
		},
	})
}

func handleHideTag(client *whatsmeow.Client, v *events.Message, args []string) {
	if !v.Info.IsGroup {
		msg := `╔════════════════╗
║ ❌ GROUP ONLY
╠════════════════
║ This command
║ works only in
║ group chats
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	if !isAdmin(client, v.Info.Chat, v.Info.Sender) && !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ DENIED
╠════════════════
║ 🔒 Admin Only
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	info, _ := client.GetGroupInfo(context.Background(), v.Info.Chat)
	mentions := []string{}
	text := strings.Join(args, " ")

	if text == "" {
		text = "🔔 Hidden Tag"
	}

	for _, p := range info.Participants {
		mentions = append(mentions, p.JID.String())
	}

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				MentionedJID: mentions,
			},
		},
	})
}

func handleGroup(client *whatsmeow.Client, v *events.Message, args []string) {
	if !v.Info.IsGroup {
		msg := `╔════════════════╗
║ ❌ GROUP ONLY
╠════════════════
║ This command
║ works only in
║ group chats
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	if !isAdmin(client, v.Info.Chat, v.Info.Sender) && !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ DENIED
╠════════════════
║ 🔒 Admin Only
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	if len(args) == 0 {
		msg := `╔════════════════╗
║ ⚙️ SETTINGS
╠════════════════
║ Commands:
║
║ 🔒 .group close
║    Close group
║
║ 🔓 .group open
║    Open group
║
║ 🔗 .group link
║    Get link
║
║ 🔄 .group revoke
║    Revoke link
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	switch strings.ToLower(args[0]) {
	case "close":
		client.SetGroupAnnounce(context.Background(), v.Info.Chat, true)
		msg := `╔════════════════╗
║ 🔒 CLOSED
╠════════════════
║ Only admins
║ can send now
╚════════════════`
		replyMessage(client, v, msg)

	case "open":
		client.SetGroupAnnounce(context.Background(), v.Info.Chat, false)
		msg := `╔════════════════╗
║ 🔓 OPENED
╠════════════════
║ All members
║ can send now
╚════════════════`
		replyMessage(client, v, msg)

	case "link":
		code, _ := client.GetGroupInviteLink(context.Background(), v.Info.Chat, false)
		msg := fmt.Sprintf(`╔════════════════╗
║ 🔗 LINK
╠════════════════
║ https://chat.
║ whatsapp.com/
║ %s
╚════════════════`, code)
		replyMessage(client, v, msg)

	case "revoke":
		client.GetGroupInviteLink(context.Background(), v.Info.Chat, true)
		msg := `╔════════════════╗
║ 🔄 REVOKED
╠════════════════
║ Old link is
║ now invalid
║ Use .group link
║ for new one
╚════════════════`
		replyMessage(client, v, msg)

	default:
		msg := `╔════════════════╗
║ ❌ INVALID
╠════════════════
║ Use: close,
║ open, link, or
║ revoke
╚════════════════`
		replyMessage(client, v, msg)
	}
}

func handleDelete(client *whatsmeow.Client, v *events.Message) {
	if !v.Info.IsGroup {
		return
	}

	if !isAdmin(client, v.Info.Chat, v.Info.Sender) && !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ DENIED
╠════════════════
║ 🔒 Admin Only
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	if v.Message.ExtendedTextMessage == nil {
		msg := `╔════════════════╗
║ ⚠️ INVALID
╠════════════════
║ Reply to a
║ message to
║ delete it
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	ctx := v.Message.ExtendedTextMessage.ContextInfo
	if ctx == nil || ctx.StanzaID == nil {
		return
	}

	client.RevokeMessage(context.Background(), v.Info.Chat, *ctx.StanzaID)

	msg := `╔════════════════╗
║ 🗑️ DELETED
╠════════════════
║ ✅ Removed
╚════════════════`
	replyMessage(client, v, msg)
}

func groupAction(client *whatsmeow.Client, v *events.Message, args []string, action string) {
	if !v.Info.IsGroup {
		msg := `╔════════════════╗
║ ❌ GROUP ONLY
╠════════════════
║ This command
║ works only in
║ group chats
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	if !isAdmin(client, v.Info.Chat, v.Info.Sender) && !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ DENIED
╠════════════════
║ 🔒 Admin Only
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	var targetJID types.JID
	if len(args) > 0 {
		num := strings.TrimSpace(args[0])
		num = strings.ReplaceAll(num, "+", "")
		if !strings.Contains(num, "@") {
			num = num + "@s.whatsapp.net"
		}
		jid, err := types.ParseJID(num)
		if err != nil {
			msg := `╔════════════════╗
║ ❌ INVALID
╠════════════════
║ Invalid number
╚════════════════`
			replyMessage(client, v, msg)
			return
		}
		targetJID = jid
	} else if v.Message.ExtendedTextMessage != nil && v.Message.ExtendedTextMessage.ContextInfo != nil {
		ctx := v.Message.ExtendedTextMessage.ContextInfo
		if ctx.Participant != nil {
			jid, _ := types.ParseJID(*ctx.Participant)
			targetJID = jid
		} else if len(ctx.MentionedJID) > 0 {
			jid, _ := types.ParseJID(ctx.MentionedJID[0])
			targetJID = jid
		}
	}

	if targetJID.User == "" {
		msg := `╔════════════════╗
║ ⚠️ NO USER
╠════════════════
║ Mention or
║ reply to user
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	if targetJID.User == v.Info.Sender.User && action == "remove" {
		msg := `╔════════════════╗
║ ❌ INVALID
╠════════════════
║ Cannot kick
║ yourself
╚════════════════`
		replyMessage(client, v, msg)
		return
	}

	var actionText, actionEmoji string
	var participantChange whatsmeow.ParticipantChange

	switch action {
	case "remove":
		participantChange = whatsmeow.ParticipantChangeRemove
		actionText = "Kicked"
		actionEmoji = "👢"
	case "promote":
		participantChange = whatsmeow.ParticipantChangePromote
		actionText = "Promoted"
		actionEmoji = "⬆️"
	case "demote":
		participantChange = whatsmeow.ParticipantChangeDemote
		actionText = "Demoted"
		actionEmoji = "⬇️"
	}

	client.UpdateGroupParticipants(context.Background(), v.Info.Chat, []types.JID{targetJID}, participantChange)

	msg := fmt.Sprintf(`╔════════════════╗
║ %s %s
╠════════════════
║ User: @%s
║ ✅ Done
╚════════════════`, actionEmoji, strings.ToUpper(actionText), targetJID.User)

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(msg),
			ContextInfo: &waProto.ContextInfo{
				MentionedJID: []string{targetJID.String()},
				StanzaID:     proto.String(v.Info.ID),
				Participant:  proto.String(v.Info.Sender.String()),
			},
		},
	})
}






type BotLIDInfo struct {
	Phone       string    `json:"phone" bson:"phone"`
	LID         string    `json:"lid" bson:"lid"`
	Platform    string    `json:"platform" bson:"platform"`
	SessionID   string    `json:"sessionId" bson:"sessionId"`
	ExtractedAt time.Time `json:"extractedAt" bson:"extractedAt"`
	LastUpdated time.Time `json:"lastUpdated" bson:"lastUpdated"`
}

type LIDDatabase struct {
	Timestamp time.Time             `json:"timestamp"`
	Count     int                   `json:"count"`
	Bots      map[string]BotLIDInfo `json:"bots"`
}

var (
	lidCache      = make(map[string]string) 
	lidCacheMutex sync.RWMutex
	lidDataFile   = "./lid_data.json"
	lidLogFile    = "./lid_extractor.log"
)






func getCleanNumber(jidStr string) string {
	if jidStr == "" {
		return ""
	}
	parts := strings.Split(jidStr, "@")
	userPart := parts[0]
	if strings.Contains(userPart, ":") {
		userPart = strings.Split(userPart, ":")[0]
	}
	return strings.TrimSpace(userPart)
}


func getBotPhoneNumber(client *whatsmeow.Client) string {
	if client.Store.ID == nil || client.Store.ID.IsEmpty() {
		return ""
	}
	return getCleanNumber(client.Store.ID.User)
}


func getSenderPhoneNumber(sender types.JID) string {
	if sender.IsEmpty() {
		return ""
	}
	return getCleanNumber(sender.User)
}






func runLIDExtractor() error {

	
	_, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("node.js not available")
	}

	
	extractorPath := "./lid-extractor.js"
	if _, err := os.Stat(extractorPath); os.IsNotExist(err) {
		return fmt.Errorf("extractor script not found")
	}

	
	cmd := exec.Command("node", extractorPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	startTime := time.Now()

	if err := cmd.Run(); err != nil {
	}

	duration := time.Since(startTime).Seconds()

	return nil
}






func loadLIDData() error {
	lidCacheMutex.Lock()
	defer lidCacheMutex.Unlock()

	
	if _, err := os.Stat(lidDataFile); os.IsNotExist(err) {
		return nil
	}

	
	data, err := os.ReadFile(lidDataFile)
	if err != nil {
		return fmt.Errorf("failed to read LID data: %v", err)
	}

	
	var lidDB LIDDatabase
	if err := json.Unmarshal(data, &lidDB); err != nil {
		return fmt.Errorf("failed to parse LID data: %v", err)
	}

	
	lidCache = make(map[string]string)
	for phone, botInfo := range lidDB.Bots {
		lidCache[phone] = botInfo.LID
	}


	
	if len(lidCache) > 0 {
		for phone, lid := range lidCache {
		}
	}

	return nil
}






func saveLIDToRedis(botInfo BotLIDInfo) error {
	if rdb == nil {
		return fmt.Errorf("redis not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	
	botInfo.LastUpdated = time.Now()
	jsonData, err := json.Marshal(botInfo)
	if err != nil {
		return fmt.Errorf("marshal failed: %v", err)
	}

	
	err = rdb.HSet(ctx, "bot_lids_store", botInfo.Phone, jsonData).Err()
	if err != nil {
		return fmt.Errorf("redis hset failed: %v", err)
	}

	return nil
}


func loadLIDsFromRedis() error {
	if rdb == nil {
		return fmt.Errorf("redis not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	
	data, err := rdb.HGetAll(ctx, "bot_lids_store").Result()
	if err != nil {
		return fmt.Errorf("redis hgetall failed: %v", err)
	}

	lidCacheMutex.Lock()
	defer lidCacheMutex.Unlock()

	count := 0
	for _, val := range data {
		var botInfo BotLIDInfo
		if err := json.Unmarshal([]byte(val), &botInfo); err != nil {
			continue
		}
		lidCache[botInfo.Phone] = botInfo.LID
		count++
	}

	if count > 0 {
	}

	return nil
}


func syncLIDsToRedis() error {
	
	data, err := os.ReadFile(lidDataFile)
	if err != nil {
		return nil 
	}

	var lidDB LIDDatabase
	if err := json.Unmarshal(data, &lidDB); err != nil {
		return err
	}

	
	for _, botInfo := range lidDB.Bots {
		if err := saveLIDToRedis(botInfo); err != nil {
		}
	}

	return nil
}






func getLIDForPhone(phone string) string {
	lidCacheMutex.RLock()
	defer lidCacheMutex.RUnlock()

	cleanPhone := getCleanNumber(phone)
	if lid, exists := lidCache[cleanPhone]; exists {
		return lid
	}
	return ""
}


func isOwnerByLID(client *whatsmeow.Client, sender types.JID) bool {
	botPhone := getBotPhoneNumber(client)
	if botPhone == "" {
		return false
	}

	
	botLID := getLIDForPhone(botPhone)
	if botLID == "" {
		return false
	}

	
	senderPhone := getSenderPhoneNumber(sender)
	if senderPhone == "" {
		return false
	}

	
	isMatch := (senderPhone == botLID)


	return isMatch
}





func sendOwnerStatus(client *whatsmeow.Client, v *events.Message) {
	botPhone := getBotPhoneNumber(client)
	botLID := getLIDForPhone(botPhone)
	senderPhone := getSenderPhoneNumber(v.Info.Sender)
	isOwn := isOwnerByLID(client, v.Info.Sender)

	status := "❌ NOT Owner"
	icon := "🚫"
	if isOwn {
		status = "✅ YOU are Owner"
		icon = "👑"
	}

	msg := fmt.Sprintf(`╔════════════════════╗
║ %s OWNER STATUS
╠════════════════════╣
║ 📱 Bot: %s
║ 🆔 LID: %s
║ 👤 You: %s
║ 
║ %s
╠════════════════════╣
║ 🔐 LID-Based Verification
╚════════════════════╝`,
		icon, botPhone, botLID, senderPhone, status)

	sendReplyMessage(client, v, msg)
}






func InitLIDSystem() {

	
	if err := loadLIDsFromRedis(); err != nil {
	}

	
	if err := runLIDExtractor(); err != nil {
	}

	
	if err := loadLIDData(); err != nil {
	}

	
	if rdb != nil {
		if err := syncLIDsToRedis(); err != nil {
		}
	}

	
	lidCacheMutex.RLock()
	count := len(lidCache)
	lidCacheMutex.RUnlock()

	if count > 0 {
	} else {
	}

	
	if count == 0 {
	}
}






func OnNewPairing(client *whatsmeow.Client) {
	
	
	time.Sleep(3 * time.Second)
	
	
	if err := runLIDExtractor(); err != nil {
		return
	}
	
	
	if err := loadLIDData(); err != nil {
		return
	}
	
	
	if rdb != nil {
		syncLIDsToRedis()
	}
	
	botPhone := getBotPhoneNumber(client)
	botLID := getLIDForPhone(botPhone)
	
	if botLID != "" {
	}
}






func canExecuteCommand(client *whatsmeow.Client, v *events.Message, cmd string) bool {
	
	if isOwnerByLID(client, v.Info.Sender) {
		return true
	}

	
	if !v.Info.IsGroup {
		return true
	}

	
	s := getGroupSettings(v.Info.Chat.String())

	if s.Mode == "private" {
		return false
	}

	if s.Mode == "admin" {
		return isGroupAdmin(client, v.Info.Chat, v.Info.Sender)
	}

	return true 
}


func isGroupAdmin(client *whatsmeow.Client, chat, user types.JID) bool {
	info, err := client.GetGroupInfo(context.Background(), chat)
	if err != nil {
		return false
	}

	userPhone := getSenderPhoneNumber(user)

	for _, p := range info.Participants {
		participantPhone := getSenderPhoneNumber(p.JID)
		if participantPhone == userPhone && (p.IsAdmin || p.IsSuperAdmin) {
			return true
		}
	}

	return false
}


var (
	client           *whatsmeow.Client
	container        *sqlstore.Container
	dbContainer      *sqlstore.Container  
	rdb              *redis.Client 
	ctx              = context.Background()
	persistentUptime int64
    groupCache = make(map[string]*GroupSettings)
    cacheMutex sync.RWMutex
	upgrader         = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	wsClients = make(map[*websocket.Conn]bool)
	botCleanIDCache = make(map[string]string)
	botPrefixes     = make(map[string]string)
	prefixMutex     sync.RWMutex
	clientsMutex    sync.RWMutex
	activeClients   = make(map[string]*whatsmeow.Client)
	globalClient *whatsmeow.Client 
	ytCache         = make(map[string]YTSession) 
	ytDownloadCache = make(map[string]YTState)
)


func initRedis() {
	redisURL := os.Getenv("REDIS_URL")
	
	if redisURL == "" {
		redisURL = "redis:
	} else {
		
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
	}

	rdb = redis.NewClient(opt)

	
	_, err = rdb.Ping(ctx).Result()
	if err != nil {
	}
}

func main() {

	
	initRedis()
	loadPersistentUptime()
	startPersistentUptimeTracker()

	
	dbURL := os.Getenv("DATABASE_URL")
	dbType := "postgres"
	if dbURL == "" {
		dbType = "sqlite3"
		dbURL = "file:impossible.db?_foreign_keys=on"
	}

	dbLog := waLog.Stdout("Database", "ERROR", true)
	var err error
	container, err = sqlstore.New(context.Background(), dbType, dbURL, dbLog)
	if err != nil {
	}
	dbContainer = container

	
	StartAllBots(container)

	
	InitLIDSystem()
	

	
	http.HandleFunc("/", serveHTML)
	http.HandleFunc("/pic.png", servePicture)
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/api/pair", handlePairAPI)
	http.HandleFunc("/link/pair/", handlePairAPILegacy)
	http.HandleFunc("/link/delete", handleDeleteSession)
	http.HandleFunc("/del/all", handleDelAllAPI)
	http.HandleFunc("/del/", handleDelNumberAPI)

	port := os.Getenv("PORT")
	if port == "" { port = "8080" }

	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
		}
	}()

	
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	clientsMutex.Lock()
	for id, activeClient := range activeClients {
		activeClient.Disconnect()
	}
	clientsMutex.Unlock()
}


func ConnectNewSession(device *store.Device) {
	
	rawID := device.ID.User
	cleanID := getCleanID(rawID)
	
	
	clientsMutex.Lock()
	botCleanIDCache[rawID] = cleanID
	clientsMutex.Unlock()

	
	
	p, err := rdb.Get(ctx, "prefix:"+cleanID).Result()
	if err != nil {
		p = "." 
	}
	
	
	prefixMutex.Lock()
	botPrefixes[cleanID] = p
	prefixMutex.Unlock()

	
	clientsMutex.RLock()
	_, exists := activeClients[cleanID]
	clientsMutex.RUnlock()
	if exists {
		return
	}

	
	clientLog := waLog.Stdout("Client", "ERROR", true)
	newBotClient := whatsmeow.NewClient(device, clientLog)
	
	
	newBotClient.AddEventHandler(func(evt interface{}) {
		handler(newBotClient, evt)
	})

	
	err = newBotClient.Connect()
	if err != nil {
		return
	}

	
	clientsMutex.Lock()
	activeClients[cleanID] = newBotClient
	clientsMutex.Unlock()

	
}


func updatePrefixDB(botID string, newPrefix string) {
	prefixMutex.Lock()
	botPrefixes[botID] = newPrefix
	prefixMutex.Unlock()

	
	err := rdb.Set(ctx, "prefix:"+botID, newPrefix, 0).Err()
	if err != nil {
	}
}




func serveHTML(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

func servePicture(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "pic.png")
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	wsClients[conn] = true
	defer delete(wsClients, conn)

	status := map[string]interface{}{
		"connected": client != nil && client.IsConnected(),
		"session":   client != nil && client.Store.ID != nil,
	}
	conn.WriteJSON(status)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func broadcastWS(data interface{}) {
	for conn := range wsClients {
		conn.WriteJSON(data)
	}
}


func handleDelAllAPI(w http.ResponseWriter, r *http.Request) {
	
	
	clientsMutex.Lock()
	for id, c := range activeClients {
		c.Disconnect()
		delete(activeClients, id)
	}
	clientsMutex.Unlock()

	
	devices, _ := container.GetAllDevices(context.Background())
	for _, dev := range devices {
		dev.Delete(context.Background())
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"success":true, "message":"All sessions wiped from DB and memory"}`)
}


func handleDelNumberAPI(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, `{"error":"Number required"}`, 400)
		return
	}
	targetNum := parts[2]

	
	clientsMutex.Lock()
	if c, ok := activeClients[getCleanID(targetNum)]; ok {
		c.Disconnect()
		delete(activeClients, getCleanID(targetNum))
	}
	clientsMutex.Unlock()

	
	devices, _ := container.GetAllDevices(context.Background())
	deleted := false
	for _, dev := range devices {
		if getCleanID(dev.ID.User) == getCleanID(targetNum) {
			dev.Delete(context.Background())
			deleted = true
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if deleted {
		fmt.Fprintf(w, `{"success":true, "message":"Session deleted for %s"}`, targetNum)
	} else {
		fmt.Fprintf(w, `{"success":false, "message":"No session found for %s"}`, targetNum)
	}
}


func handlePairAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, `{"error":"Method not allowed"}`, 405)
		return
	}

	var req struct {
		Number string `json:"number"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid JSON"}`, 400)
		return
	}

	
	number := strings.TrimSpace(req.Number)
	number = strings.ReplaceAll(number, "+", "")
	number = strings.ReplaceAll(number, " ", "")
	number = strings.ReplaceAll(number, "-", "")
	cleanNum := getCleanID(number)


	
	devices, _ := container.GetAllDevices(context.Background())
	for _, dev := range devices {
		if getCleanID(dev.ID.User) == cleanNum {
			
			
			clientsMutex.Lock()
			if c, ok := activeClients[cleanNum]; ok {
				c.Disconnect()
				delete(activeClients, cleanNum)
			}
			clientsMutex.Unlock()
			
			
			dev.Delete(context.Background())
		}
	}

	
	newDevice := container.NewDevice()
	tempClient := whatsmeow.NewClient(newDevice, waLog.Stdout("Pairing", "INFO", true))
	
	tempClient.AddEventHandler(func(evt interface{}) {
        handler(tempClient, evt)
    })

	err := tempClient.Connect()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), 500)
		return
	}

	
	time.Sleep(5 * time.Second)

	code, err := tempClient.PairPhone(context.Background(), number, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		tempClient.Disconnect()
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), 500)
		return
	}


	broadcastWS(map[string]interface{}{
		"event": "pairing_code",
		"code":  code,
	})

	go func() {
		for i := 0; i < 60; i++ {
			time.Sleep(1 * time.Second)
			if tempClient.Store.ID != nil {
				clientsMutex.Lock()
				activeClients[cleanNum] = tempClient
				clientsMutex.Unlock()
				return
			}
		}
		tempClient.Disconnect()
	}()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"success":true,"code":"%s"}`, code)
}


func handlePairAPILegacy(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, `{"error":"Invalid URL"}`, 400)
		return
	}

	number := strings.TrimSpace(parts[3])
	number = strings.ReplaceAll(number, "+", "")
	number = strings.ReplaceAll(number, " ", "")
	number = strings.ReplaceAll(number, "-", "")

	if len(number) < 10 {
		http.Error(w, `{"error":"Invalid number"}`, 400)
		return
	}


	if client != nil && client.IsConnected() {
		client.Disconnect()
		time.Sleep(10 * time.Second)
	}

	newDevice := container.NewDevice()
	tempClient := whatsmeow.NewClient(newDevice, waLog.Stdout("Pairing", "INFO", true))
	
	SetGlobalClient(tempClient)
	tempClient.AddEventHandler(func(evt interface{}) {
        handler(tempClient, evt)
    })

	err := tempClient.Connect()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), 500)
		return
	}

	time.Sleep(10 * time.Second)

	code, err := tempClient.PairPhone(
		context.Background(),
		number,
		true,
		whatsmeow.PairClientChrome,
		"Chrome (Linux)",
	)

	if err != nil {
		tempClient.Disconnect()
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), 500)
		return
	}


	go func() {
		for i := 0; i < 60; i++ {
			time.Sleep(1 * time.Second)
			if tempClient.Store.ID != nil {
				client = tempClient
				
				OnNewPairing(client)
				
				return
			}
		}
		tempClient.Disconnect()
	}()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"success":true,"code":"%s"}`, code)
}

func handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	if client != nil && client.IsConnected() {
		client.Disconnect()
	}

	devices, _ := container.GetAllDevices(context.Background())
	for _, device := range devices {
		device.Delete(context.Background())
	}

	broadcastWS(map[string]interface{}{
		"event":     "session_deleted",
		"connected": false,
	})

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"success":true,"message":"Session deleted"}`)
}

func StartAllBots(container *sqlstore.Container) {
	dbContainer = container
	devices, err := container.GetAllDevices(context.Background())
	if err != nil {
		return
	}

	seenNumbers := make(map[string]bool)

	for _, device := range devices {
		botNum := getCleanID(device.ID.User)
		if seenNumbers[botNum] { continue }
		seenNumbers[botNum] = true

		go func(dev *store.Device) {
			defer func() {
				if r := recover(); r != nil {
				}
			}()
			ConnectNewSession(dev)
		}(device)
		time.Sleep(5 * time.Second)
	}
	go monitorNewSessions(container)
}


func loadPersistentUptime() {
	if rdb != nil {
		val, err := rdb.Get(ctx, "total_uptime").Int64()
		if err == nil { persistentUptime = val }
	}
}


func startPersistentUptimeTracker() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			persistentUptime += 60
			if rdb != nil {
				rdb.Set(ctx, "total_uptime", persistentUptime, 0)
			}
		}
	}()
}


func SetGlobalClient(c *whatsmeow.Client) {
	globalClient = c
}


func saveGroupSettings(s *GroupSettings) {
	cacheMutex.Lock()
	groupCache[s.ChatID] = s
	cacheMutex.Unlock()
}

func monitorNewSessions(container *sqlstore.Container) {
	ticker := time.NewTicker(60 * time.Second) 
	defer ticker.Stop()

	for range ticker.C {
		
		devices, err := container.GetAllDevices(context.Background())
		if err != nil {
			continue
		}

		for _, device := range devices {
			botID := getCleanID(device.ID.User)
			
			
			clientsMutex.RLock()
			_, exists := activeClients[botID]
			clientsMutex.RUnlock()

			
			if !exists {
				go ConnectNewSession(device)
				time.Sleep(5 * time.Second) 
			}
		}
	}
}




type BotSettings struct {
	Prefix     string `json:"prefix"`
	SelfMode   bool   `json:"self_mode"`
	AutoStatus bool   `json:"auto_status"`
	OnlyGroup  bool   `json:"only_group"`
}


func SaveAllSettings(rdb *redis.Client, botID string, settings BotSettings) {
	
	data, err := json.Marshal(settings)
	if err != nil {
		return
	}

	
	key := fmt.Sprintf("settings:%s", botID)
	err = rdb.Set(ctx, key, data, 0).Err() 
	if err != nil {
	} else {
	}
}


func LoadAllSettings(rdb *redis.Client, botID string) BotSettings {
	key := fmt.Sprintf("settings:%s", botID)
	val, err := rdb.Get(ctx, key).Result()

	var settings BotSettings
	if err == redis.Nil {
		
		return BotSettings{Prefix: ".", SelfMode: false, AutoStatus: true}
	} else if err != nil {
		return BotSettings{Prefix: "."}
	}

	
	err = json.Unmarshal([]byte(val), &settings)
	if err != nil {
	}
	
	return settings
}


type GroupSecurity struct {
	AntiLink   bool `json:"anti_link"`
	AllowAdmin bool `json:"allow_admin"` 
}


func SaveGroupSecurity(rdb *redis.Client, botLID string, groupID string, data GroupSecurity) {
	key := fmt.Sprintf("sec:%s:%s", botLID, groupID)
	payload, _ := json.Marshal(data)
	
	err := rdb.Set(ctx, key, payload, 0).Err()
	if err != nil {
	}
}


func LoadGroupSecurity(rdb *redis.Client, botLID string, groupID string) GroupSecurity {
	key := fmt.Sprintf("sec:%s:%s", botLID, groupID)
	val, err := rdb.Get(ctx, key).Result()
	
	var data GroupSecurity
	if err != nil {
		
		return GroupSecurity{AntiLink: false, AllowAdmin: false}
	}
	
	json.Unmarshal([]byte(val), &data)
	return data
}


func finalizeSecurity(client *whatsmeow.Client, senderLID string, choice string) {
	state := setupMap[senderLID]
	if state == nil { return }

	allowAdmin := (choice == "1") 
	
	
	newConfig := GroupSecurity{
		AntiLink:   true, 
		AllowAdmin: allowAdmin,
	}

	
	SaveGroupSecurity(rdb, state.BotLID, state.GroupID, newConfig)
	
	
	delete(setupMap, senderLID)
}

func checkSecurity(client *whatsmeow.Client, v *events.Message) {
	if !v.Info.IsGroup {
		return
	}

	s := getGroupSettings(v.Info.Chat.String())
	if s.Mode == "private" {
		return
	}

	
	if s.Antilink && containsLink(getText(v.Message)) {
		
		takeSecurityAction(client, v, s, s.AntilinkAction, "Link detected")
		return
	}

	
	if s.AntiPic && v.Message.ImageMessage != nil {
		takeSecurityAction(client, v, s, "delete", "Image not allowed")
		return
	}

	
	if s.AntiVideo && v.Message.VideoMessage != nil {
		takeSecurityAction(client, v, s, "delete", "Video not allowed")
		return
	}

	
	if s.AntiSticker && v.Message.StickerMessage != nil {
		takeSecurityAction(client, v, s, "delete", "Sticker not allowed")
		return
	}
}

func containsLink(text string) bool {
	if text == "" {
		return false
	}

	text = strings.ToLower(text)
	linkPatterns := []string{
		"http://", "https://", "www.",
		"chat.whatsapp.com/", "t.me/", "youtube.com/",
		"youtu.be/", "instagram.com/", "fb.com/",
		"facebook.com/", "twitter.com/", "x.com/",
	}

	for _, pattern := range linkPatterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}

	return false
}

func takeSecurityAction(client *whatsmeow.Client, v *events.Message, s *GroupSettings, action, reason string) {
	switch action {
	case "delete":
		
		_, err := client.SendMessage(context.Background(), v.Info.Chat, client.BuildRevoke(v.Info.Chat, v.Info.Sender, v.Info.ID))
		if err != nil {
			msg := `╔════════════════╗
║ ❌ DELETE FAILED
╠════════════════╣
║ Bot needs admin
╚════════════════╝`
			replyMessage(client, v, msg)
			return
		}


		msg := fmt.Sprintf(`╔════════════════╗
║ 🚫 DELETED
╠════════════════╣
║ Reason: %s
║ User: @%s
╚════════════════╝`, reason, v.Info.Sender.User)
		
		senderStr := v.Info.Sender.String()
		client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text: proto.String(msg),
				ContextInfo: &waProto.ContextInfo{
					MentionedJID: []string{senderStr},
					StanzaID:     proto.String(v.Info.ID),
					Participant:  proto.String(senderStr),
				},
			},
		})

	case "deletekick":
		
		_, err := client.SendMessage(context.Background(), v.Info.Chat, client.BuildRevoke(v.Info.Chat, v.Info.Sender, v.Info.ID))
		if err != nil {
			msg := `╔════════════════╗
║ ❌ DELETE FAILED
╠════════════════╣
║ Bot needs admin
╚════════════════╝`
			replyMessage(client, v, msg)
			return
		}


		_, err = client.UpdateGroupParticipants(context.Background(), v.Info.Chat,
			[]types.JID{v.Info.Sender}, whatsmeow.ParticipantChangeRemove)
		
		if err != nil {
			msg := `╔════════════════╗
║ ⚠️ KICK FAILED
╠════════════════╣
║ Bot needs admin
╚════════════════╝`
			replyMessage(client, v, msg)
			return
		}

		
		msg := fmt.Sprintf(`╔════════════════╗
║ 👢 KICKED
╠════════════════╣
║ Reason: %s
║ User: @%s
║ Action: Delete+Kick
╚════════════════╝`, reason, v.Info.Sender.User)
		
		senderStr := v.Info.Sender.String()
		client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text: proto.String(msg),
				ContextInfo: &waProto.ContextInfo{
					MentionedJID: []string{senderStr},
				},
			},
		})

	case "deletewarn":
		senderKey := v.Info.Sender.String()
		s.Warnings[senderKey]++
		warnCount := s.Warnings[senderKey]

		
		_, err := client.SendMessage(context.Background(), v.Info.Chat, client.BuildRevoke(v.Info.Chat, v.Info.Sender, v.Info.ID))
		if err != nil {
			msg := `╔════════════════╗
║ ❌ DELETE FAILED
╠════════════════╣
║ Bot needs admin
╚════════════════╝`
			replyMessage(client, v, msg)
			return
		}


		if warnCount >= 3 {
			_, err := client.UpdateGroupParticipants(context.Background(), v.Info.Chat,
				[]types.JID{v.Info.Sender}, whatsmeow.ParticipantChangeRemove)
			
			if err != nil {
				msg := `╔════════════════╗
║ ⚠️ KICK FAILED
╠════════════════╣
║ Bot needs admin
╚════════════════╝`
				replyMessage(client, v, msg)
				return
			}


			delete(s.Warnings, senderKey)
			
			msg := fmt.Sprintf(`╔════════════════╗
║ 🚫 KICKED
╠════════════════╣
║ User: @%s
║ Warning: 3/3
║ Kicked Out
╚════════════════╝`, v.Info.Sender.User)
			
			senderStr := v.Info.Sender.String()
			client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
				ExtendedTextMessage: &waProto.ExtendedTextMessage{
					Text: proto.String(msg),
					ContextInfo: &waProto.ContextInfo{
						MentionedJID: []string{senderStr},
					},
				},
			})
		} else {
			msg := fmt.Sprintf(`╔════════════════╗
║ ⚠️ WARNING
╠════════════════╣
║ User: @%s
║ Count: %d/3
║ Reason: %s
║ 3 = Kick
╚════════════════╝`, v.Info.Sender.User, warnCount, reason)
			
			senderStr := v.Info.Sender.String()
			client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
				ExtendedTextMessage: &waProto.ExtendedTextMessage{
					Text: proto.String(msg),
					ContextInfo: &waProto.ContextInfo{
						MentionedJID: []string{senderStr},
						StanzaID:     proto.String(v.Info.ID),
						Participant:  proto.String(senderStr),
					},
				},
			})
		}

		saveGroupSettings(s)
	}
}

func onResponse(client *whatsmeow.Client, v *events.Message, choice string) {
	senderID := v.Info.Sender.String()
	state, exists := setupMap[senderID]

	
	if !exists { return }

	
	if v.Message.GetExtendedTextMessage().GetContextInfo() == nil {
		return 
	}

	
	quotedID := v.Message.ExtendedTextMessage.ContextInfo.GetStanzaID() 
	if quotedID != state.BotMsgID {
		return 
	}

	
	key := fmt.Sprintf("group:sec:%s:%s:%s", state.BotLID, state.GroupID, state.Type)
	rdb.Set(context.Background(), key, choice, 0)

	
	replyMessage(client, v, "✅ Setting Saved Successfully!")
	delete(setupMap, senderID)
}

func startSecuritySetup(client *whatsmeow.Client, v *events.Message, secType string) {
	
	if !v.Info.IsGroup {
		replyMessage(client, v, "╔════════════════╗\n║ ❌ GROUP ONLY\n╚════════════════╝")
		return
	}

	
	isAdmin := false
	groupInfo, _ := client.GetGroupInfo(context.Background(), v.Info.Chat)
	if groupInfo != nil {
		for _, p := range groupInfo.Participants {
			if p.JID.User == v.Info.Sender.User && (p.IsAdmin || p.IsSuperAdmin) {
				isAdmin = true; break
			}
		}
	}
	if !isAdmin && !isOwner(client, v.Info.Sender) {
		replyMessage(client, v, "╔════════════════╗\n║ 👮 ADMIN ONLY\n╚════════════════╝")
		return
	}

	
	
	cleanSenderLID := v.Info.Sender.User 
	groupID := v.Info.Chat.String()
	botUniqueLID := getBotLIDFromDB(client) 

	msgText := fmt.Sprintf(`╔════════════════╗
║ 🛡️ %s (1/2)
╠════════════════╣
║ Allow Admins?
║ 1️⃣ YES | 2️⃣ NO
╚════════════════╝`, strings.ToUpper(secType))

	
	resp, err := client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{Text: proto.String(msgText)},
	})

	if err != nil {
		return 
	}

	
	mapKey := resp.ID 


	
	setupMap[mapKey] = &SetupState{
		Type:     secType,
		Stage:    1,
		GroupID:  groupID,
		User:     cleanSenderLID, 
		BotLID:   botUniqueLID,
		BotMsgID: resp.ID,
	}

	go func() {
		time.Sleep(2 * time.Minute)
		delete(setupMap, mapKey)
	}()
}

func handleSetupResponse(client *whatsmeow.Client, v *events.Message) {
	
	extMsg := v.Message.GetExtendedTextMessage()
	if extMsg == nil || extMsg.ContextInfo == nil { return }

	quotedID := extMsg.ContextInfo.GetStanzaID()
	incomingLID := v.Info.Sender.User 
	botLID := getBotLIDFromDB(client)

	
	state, exists := setupMap[quotedID]
	if !exists { return }

	
	if state.BotLID != botLID { return }

	

	
	if state.User != incomingLID {
		
	}


	txt := strings.TrimSpace(getText(v.Message))
	s := getGroupSettings(state.GroupID)

	
	if state.Stage == 1 {
		if txt == "1" { s.AntilinkAdmin = true } else if txt == "2" { s.AntilinkAdmin = false } else { return }
		
		delete(setupMap, quotedID) 

		state.Stage = 2
		nextMsg := fmt.Sprintf(`╔════════════════╗
║ ⚡ %s (2/2)
╠════════════════╣
║ 1️⃣ DELETE ONLY
║ 2️⃣ DELETE + KICK
║ 3️⃣ DELETE + WARN
╚════════════════╝`, strings.ToUpper(state.Type))

		resp, _ := client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{Text: proto.String(nextMsg)},
		})
		
		state.BotMsgID = resp.ID 
		setupMap[resp.ID] = state 
		return
	}

	
	if state.Stage == 2 {
		var actionText string
		switch txt {
		case "1": s.AntilinkAction = "delete"; actionText = "Delete Only"
		case "2": s.AntilinkAction = "deletekick"; actionText = "Delete + Kick"
		case "3": s.AntilinkAction = "deletewarn"; actionText = "Delete + Warn"
		default: return
		}

		applySecurityFinal(s, state.Type, true)
		saveGroupSettings(s)
		delete(setupMap, quotedID) 

		adminBypass := "YES ✅"; if !s.AntilinkAdmin { adminBypass = "NO ❌" }
		finalMsg := fmt.Sprintf(`╔════════════════╗
║ ✅ %s ENABLED
╠════════════════╣
║ Admin Bypass: %s
║ Action: %s
╚════════════════╝`, strings.ToUpper(state.Type), adminBypass, actionText)

		replyMessage(client, v, finalMsg)
	}
}


func applySecurityFinal(s *GroupSettings, t string, val bool) {
	switch t {
	case "antilink": s.Antilink = val
	case "antipic": s.AntiPic = val
	case "antivideo": s.AntiVideo = val
	case "antisticker": s.AntiSticker = val
	}
}


func participantIsAdmin(p types.GroupParticipant) bool {
	return p.IsAdmin || p.IsSuperAdmin
}

func handleGroupEvents(client *whatsmeow.Client, evt interface{}) {
	switch v := evt.(type) {
	case *events.GroupInfo:
		handleGroupInfoChange(client, v)
	}
}

func handleGroupInfoChange(client *whatsmeow.Client, v *events.GroupInfo) {
    // 👇 یہ نیا کوڈ ایڈ کریں 👇
    if v.JID.String() != "" {
        // اگر گروپ میں کوئی تبدیلی ہوئی ہے تو پرانا کیش اڑا دیں
        adminCacheMutex.Lock()
        delete(adminCache, v.JID.String())
        adminCacheMutex.Unlock()
    }
    // 👆 یہاں تک 👆

    if v.JID.IsEmpty() {
        return
    }
    // ... باقی کوڈ ویسا ہی رہنے دیں ...


	
	if v.Leave != nil && len(v.Leave) > 0 {
		for _, left := range v.Leave {
			sender := v.Sender 
			leftStr := left.String()
			senderStr := sender.String()

			
			if sender.User == left.User {
				msg := fmt.Sprintf(`╔════════════════╗
║ 👋 MEMBER LEFT
╠════════════════╣
║ 👤 User: @%s
║ 📉 Status: Self Leave
╚════════════════╝`, left.User)

				client.SendMessage(context.Background(), v.JID, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text: proto.String(msg),
						ContextInfo: &waProto.ContextInfo{
							MentionedJID: []string{leftStr},
						},
					},
				})
			} else {
				
				msg := fmt.Sprintf(`╔════════════════╗
║ 👢 MEMBER KICKED
╠════════════════╣
║ 👤 User: @%s
║ 👮 By: @%s
╚════════════════╝`, left.User, sender.User)

				client.SendMessage(context.Background(), v.JID, &waProto.Message{
					ExtendedTextMessage: &waProto.ExtendedTextMessage{
						Text: proto.String(msg),
						ContextInfo: &waProto.ContextInfo{
							MentionedJID: []string{leftStr, senderStr}, 
						},
					},
				})
			}
		}
	}

	
	
	
	if v.Promote != nil && len(v.Promote) > 0 {
		for _, promoted := range v.Promote {
			msg := fmt.Sprintf(`╔════════════════╗
║ 👑 PROMOTED
╠════════════════╣
║ 👤 User: @%s
║ 🎉 Congrats!
╚════════════════╝`, promoted.User)

			promotedStr := promoted.String()
			client.SendMessage(context.Background(), v.JID, &waProto.Message{
				ExtendedTextMessage: &waProto.ExtendedTextMessage{
					Text: proto.String(msg),
					ContextInfo: &waProto.ContextInfo{
						MentionedJID: []string{promotedStr},
					},
				},
			})
		}
	}

	
	if v.Demote != nil && len(v.Demote) > 0 {
		for _, demoted := range v.Demote {
			msg := fmt.Sprintf(`╔════════════════╗
║ 👤 DEMOTED
╠════════════════╣
║ 👤 User: @%s
║ 📉 Rank Removed
╚════════════════╝`, demoted.User)

			demotedStr := demoted.String()
			client.SendMessage(context.Background(), v.JID, &waProto.Message{
				ExtendedTextMessage: &waProto.ExtendedTextMessage{
					Text: proto.String(msg),
					ContextInfo: &waProto.ContextInfo{
						MentionedJID: []string{demotedStr},
					},
				},
			})
		}
	}

	
	if v.Join != nil && len(v.Join) > 0 {
		for _, joined := range v.Join {
			msg := fmt.Sprintf(`╔════════════════╗
║ 👋 JOINED
╠════════════════╣
║ 👤 User: @%s
║ 🎉 Welcome!
╚════════════════╝`, joined.User)

			joinedStr := joined.String()
			client.SendMessage(context.Background(), v.JID, &waProto.Message{
				ExtendedTextMessage: &waProto.ExtendedTextMessage{
					Text: proto.String(msg),
					ContextInfo: &waProto.ContextInfo{
						MentionedJID: []string{joinedStr},
					},
				},
			})
		}
	}
}



func toggleAlwaysOnline(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ ACCESS DENIED
╠════════════════╣
║ 🔒 Owner Only
╚════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	status := "OFF 🔴"
	statusText := "Disabled"
	dataMutex.Lock()
	data.AlwaysOnline = !data.AlwaysOnline
	if data.AlwaysOnline {
		client.SendPresence(context.Background(), types.PresenceAvailable)
		status = "ON 🟢"
		statusText = "Enabled"
	} else {
		client.SendPresence(context.Background(), types.PresenceUnavailable)
	}
	dataMutex.Unlock()

	msg := fmt.Sprintf(`╔════════════════╗
║ ⚙️ ALWAYS ONLINE
╠════════════════╣
║ 📊 Status: %s
║ 🔄 State: %s
║ ✅ Updated
╚════════════════╝`, status, statusText)

	replyMessage(client, v, msg)
}

func toggleAutoRead(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ ACCESS DENIED
╠════════════════╣
║ 🔒 Owner Only
╚════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	status := "OFF 🔴"
	statusText := "Disabled"
	dataMutex.Lock()
	data.AutoRead = !data.AutoRead
	if data.AutoRead {
		status = "ON 🟢"
		statusText = "Enabled"
	}
	dataMutex.Unlock()

	msg := fmt.Sprintf(`╔════════════════╗
║ ⚙️ AUTO READ
╠════════════════╣
║ 📊 Status: %s
║ 🔄 State: %s
║ ✅ Updated
╚════════════════╝`, status, statusText)

	replyMessage(client, v, msg)
}

func toggleAutoReact(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ ACCESS DENIED
╠════════════════╣
║ 🔒 Owner Only
╚════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	status := "OFF 🔴"
	statusText := "Disabled"
	dataMutex.Lock()
	data.AutoReact = !data.AutoReact
	if data.AutoReact {
		status = "ON 🟢"
		statusText = "Enabled"
	}
	dataMutex.Unlock()

	msg := fmt.Sprintf(`╔════════════════╗
║ ⚙️ AUTO REACT
╠════════════════╣
║ 📊 Status: %s
║ 🔄 State: %s
║ ✅ Updated
╚════════════════╝`, status, statusText)

	replyMessage(client, v, msg)
}

func toggleAutoStatus(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ ACCESS DENIED
╠════════════════╣
║ 🔒 Owner Only
╚════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	status := "OFF 🔴"
	statusText := "Disabled"
	dataMutex.Lock()
	data.AutoStatus = !data.AutoStatus
	if data.AutoStatus {
		status = "ON 🟢"
		statusText = "Enabled"
	}
	dataMutex.Unlock()

	msg := fmt.Sprintf(`╔════════════════╗
║ ⚙️ AUTO STATUS
╠════════════════╣
║ 📊 Status: %s
║ 🔄 State: %s
║ ✅ Updated
╚════════════════╝`, status, statusText)

	replyMessage(client, v, msg)
}

func toggleStatusReact(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ ACCESS DENIED
╠════════════════╣
║ 🔒 Owner Only
╚════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	status := "OFF 🔴"
	statusText := "Disabled"
	dataMutex.Lock()
	data.StatusReact = !data.StatusReact
	if data.StatusReact {
		status = "ON 🟢"
		statusText = "Enabled"
	}
	dataMutex.Unlock()

	msg := fmt.Sprintf(`╔════════════════╗
║ ⚙️ STATUS REACT
╠════════════════╣
║ 📊 Status: %s
║ 🔄 State: %s
║ ✅ Updated
╚════════════════╝`, status, statusText)

	replyMessage(client, v, msg)
}

func handleAddStatus(client *whatsmeow.Client, v *events.Message, args []string) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ ACCESS DENIED
╠════════════════╣
║ 🔒 Owner Only
╚════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	if len(args) < 1 {
		msg := `╔════════════════╗
║ ⚠️ INVALID FORMAT
╠════════════════╣
║ 📝 .addstatus <num>
║ 💡 .addstatus 923xx
╚════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	num := args[0]
	dataMutex.Lock()
	data.StatusTargets = append(data.StatusTargets, num)
	dataMutex.Unlock()

	msg := fmt.Sprintf(`╔════════════════╗
║ ✅ TARGET ADDED
╠════════════════╣
║ 📱 %s
║ 📊 Total: %d
╚════════════════╝`, num, len(data.StatusTargets))

	replyMessage(client, v, msg)
}

func handleDelStatus(client *whatsmeow.Client, v *events.Message, args []string) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ ACCESS DENIED
╠════════════════╣
║ 🔒 Owner Only
╚════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	if len(args) < 1 {
		msg := `╔════════════════╗
║ ⚠️ INVALID FORMAT
╠════════════════╣
║ 📝 .delstatus <num>
║ 💡 .delstatus 923xx
╚════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	num := args[0]
	dataMutex.Lock()
	newList := []string{}
	found := false
	for _, n := range data.StatusTargets {
		if n != num {
			newList = append(newList, n)
		} else {
			found = true
		}
	}
	data.StatusTargets = newList
	dataMutex.Unlock()

	if found {
		msg := fmt.Sprintf(`╔════════════════╗
║ ✅ TARGET REMOVED
╠════════════════╣
║ 📱 %s
║ 📊 Remaining: %d
╚════════════════╝`, num, len(data.StatusTargets))
		replyMessage(client, v, msg)
	} else {
		msg := `╔════════════════╗
║ ❌ NOT FOUND
╠════════════════╣
║ Number not in list
╚════════════════╝`
		replyMessage(client, v, msg)
	}
}

func handleListStatus(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) {
		return
	}

	dataMutex.RLock()
	targets := data.StatusTargets
	dataMutex.RUnlock()

	if len(targets) == 0 {
		msg := `╔════════════════╗
║ 📭 NO TARGETS
╠════════════════╣
║ Use .addstatus
╚════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	msg := "╔════════════════╗\n"
	msg += "║ 📜 STATUS TARGETS\n"
	msg += "╠════════════════╣\n"
	for i, t := range targets {
		msg += fmt.Sprintf("║ %d. %s\n", i+1, t)
	}
	msg += fmt.Sprintf("║ 📊 Total: %d\n", len(targets))
	msg += "╚════════════════╝"

	replyMessage(client, v, msg)
}

func handleSetPrefix(client *whatsmeow.Client, v *events.Message, args []string) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ ACCESS DENIED
╠════════════════╣
║ 🔒 Owner Only
╚════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	if len(args) < 1 {
		msg := `╔════════════════╗
║ ⚠️ INVALID FORMAT
╠════════════════╣
║ 📝 .setprefix <sym>
║ 💡 .setprefix .
║ 💡 .setprefix !
╚════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	newPrefix := args[0]
	dataMutex.Lock()
	data.Prefix = newPrefix
	dataMutex.Unlock()

	msg := fmt.Sprintf(`╔════════════════╗
║ ✅ PREFIX UPDATED
╠════════════════╣
║ 🔧 New: %s
║ 💡 Ex: %smenu
╚════════════════╝`, newPrefix, newPrefix)

	replyMessage(client, v, msg)
}

func handleMode(client *whatsmeow.Client, v *events.Message, args []string) {
	
	if !isOwner(client, v.Info.Sender) {
		msg := `╔════════════════╗
║ ❌ ACCESS DENIED
╠════════════════╣
║ 🔒 Owner Only
╚════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	
	if !v.Info.IsGroup {
		if len(args) < 1 {
			msg := `╔════════════════╗
║ ⚙️ GROUP MODE
╠════════════════╣
║ 1️⃣ public - All
║ 2️⃣ private - Off
║ 3️⃣ admin - Admin
║ 📝 .mode <type>
║ 💡 Use in group
║    to change mode
╚════════════════╝`
			replyMessage(client, v, msg)
			return
		}
	}

	
	if v.Info.IsGroup {
		if len(args) < 1 {
			msg := `╔════════════════╗
║ ⚙️ GROUP MODE
╠════════════════╣
║ 1️⃣ public - All
║ 2️⃣ private - Off
║ 3️⃣ admin - Admin
║ 📝 .mode <type>
╚════════════════╝`
			replyMessage(client, v, msg)
			return
		}

		mode := strings.ToLower(args[0])
		if mode != "public" && mode != "private" && mode != "admin" {
			msg := `╔════════════════╗
║ ❌ INVALID MODE
╠════════════════╣
║ Use: public/
║ private/admin
╚════════════════╝`
			replyMessage(client, v, msg)
			return
		}

		s := getGroupSettings(v.Info.Chat.String())
		s.Mode = mode
		saveGroupSettings(s)

		var modeDesc string
		switch mode {
		case "public":
			modeDesc = "Everyone"
		case "private":
			modeDesc = "Disabled"
		case "admin":
			modeDesc = "Admin only"
		}

		msg := fmt.Sprintf(`╔════════════════╗
║ ✅ MODE CHANGED
╠════════════════╣
║ 🛡️ %s
║ 📝 %s
║ ✅ Updated
╚════════════════╝`, strings.ToUpper(mode), modeDesc)

		replyMessage(client, v, msg)
	}
}

func handleReadAllStatus(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) {
		return
	}

	client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, time.Now(), types.NewJID("status@broadcast", types.DefaultUserServer), v.Info.Sender, types.ReceiptTypeRead)

	msg := `╔════════════════╗
║ ✅ STATUSES READ
╠════════════════╣
║ All marked read
╚════════════════╝`

	replyMessage(client, v, msg)
}



func handleToSticker(client *whatsmeow.Client, v *events.Message) {
	var quoted *waProto.Message
	if extMsg := v.Message.GetExtendedTextMessage(); extMsg != nil && extMsg.ContextInfo != nil {
		quoted = extMsg.ContextInfo.QuotedMessage
	}

	var media whatsmeow.DownloadableMessage
	isAnimated := false

	if quoted.GetImageMessage() != nil {
		media = quoted.GetImageMessage()
	} else if quoted.GetVideoMessage() != nil {
		media = quoted.GetVideoMessage()
		isAnimated = true
	} else {
		replyMessage(client, v, "❌ Reply to a Photo or Video to make a sticker.")
		return
	}

	react(client, v.Info.Chat, v.Info.ID, "✨")
	data, _ := client.Download(context.Background(), media)
	input := "temp_in"
	output := "temp_out.webp"
	os.WriteFile(input, data, 0644)

	
	if isAnimated {
		exec.Command("ffmpeg", "-y", "-i", input, "-vcodec", "libwebp", "-filter:v", "fps=fps=15,scale=512:512:force_original_aspect_ratio=decrease,pad=512:512:(ow-iw)/2:(oh-ih)/2:color=white@0", "-lossless", "1", "-loop", "0", "-preset", "default", "-an", "-vsync", "0", output).Run()
	} else {
		exec.Command("ffmpeg", "-y", "-i", input, "-vcodec", "libwebp", "-filter:v", "scale=512:512:force_original_aspect_ratio=decrease,pad=512:512:(ow-iw)/2:(oh-ih)/2:color=white@0", output).Run()
	}

	finalData, _ := os.ReadFile(output)
	up, _ := client.Upload(context.Background(), finalData, whatsmeow.MediaImage)

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		StickerMessage: &waProto.StickerMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("image/webp"),
			FileLength:    proto.Uint64(uint64(len(finalData))),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
		},
	})
	os.Remove(input); os.Remove(output)
}

func handleToImg(client *whatsmeow.Client, v *events.Message) {
	
	var stickerMsg *waProto.StickerMessage
	if extMsg := v.Message.GetExtendedTextMessage(); extMsg != nil && extMsg.ContextInfo != nil {
		stickerMsg = extMsg.ContextInfo.QuotedMessage.GetStickerMessage()
	}

	if stickerMsg == nil {
		replyMessage(client, v, "❌ *Error:* Please reply to a sticker with *.toimg*")
		return
	}

	react(client, v.Info.Chat, v.Info.ID, "🖼️")
	sendToolCard(client, v, "Media Converter", "WebP to PNG", "⏳ Processing Image...")

	
	data, err := client.Download(context.Background(), stickerMsg)
	if err != nil { return }

	input := fmt.Sprintf("in_%d.webp", time.Now().UnixNano())
	output := fmt.Sprintf("out_%d.png", time.Now().UnixNano())
	os.WriteFile(input, data, 0644)

	
	exec.Command("ffmpeg", "-y", "-i", input, output).Run()
	
	finalData, _ := os.ReadFile(output)
	up, err := client.Upload(context.Background(), finalData, whatsmeow.MediaImage)
	if err != nil { return }

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("image/png"),
			Caption:       proto.String("✅ *Converted to Image*"),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
			FileLength:    proto.Uint64(uint64(len(finalData))), 
		},
	})
	os.Remove(input); os.Remove(output)
}

func handleToMedia(client *whatsmeow.Client, v *events.Message, isGif bool) {
	var stickerMsg *waProto.StickerMessage
	if extMsg := v.Message.GetExtendedTextMessage(); extMsg != nil && extMsg.ContextInfo != nil {
		stickerMsg = extMsg.ContextInfo.QuotedMessage.GetStickerMessage()
	}

	if stickerMsg == nil || !stickerMsg.GetIsAnimated() {
		replyMessage(client, v, "❌ Please reply to an *Animated* sticker.")
		return
	}

	react(client, v.Info.Chat, v.Info.ID, "🎥")
	
	data, err := client.Download(context.Background(), stickerMsg)
	if err != nil { return }

	
	inputWebP := fmt.Sprintf("in_%d.webp", time.Now().UnixNano())
	tempGif := fmt.Sprintf("temp_%d.gif", time.Now().UnixNano())
	outputMp4 := fmt.Sprintf("out_%d.mp4", time.Now().UnixNano())

	os.WriteFile(inputWebP, data, 0644)

	
	
	cmdConvert := exec.Command("convert", inputWebP, "-coalesce", tempGif)
	if err := cmdConvert.Run(); err != nil {
		replyMessage(client, v, "❌ Failed to parse sticker animation.")
		os.Remove(inputWebP)
		return
	}

	
	cmd := exec.Command("ffmpeg", "-y",
		"-i", tempGif,          
		"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2,format=yuv420p", 
		"-c:v", "libx264",
		"-preset", "faster",
		"-crf", "26",
		"-movflags", "+faststart",
		"-pix_fmt", "yuv420p",
		"-t", "10",
		outputMp4)
	
	outLog, err := cmd.CombinedOutput()
	if err != nil {
		replyMessage(client, v, "❌ Graphics Engine failed.")
		os.Remove(inputWebP); os.Remove(tempGif)
		return
	}

	finalData, _ := os.ReadFile(outputMp4)
	up, err := client.Upload(context.Background(), finalData, whatsmeow.MediaVideo)
	if err != nil { 
		os.Remove(inputWebP); os.Remove(tempGif); os.Remove(outputMp4)
		return 
	}

	msg := &waProto.Message{
		VideoMessage: &waProto.VideoMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("video/mp4"),
			Caption:       proto.String("✅ *Converted by Impossible Media Lab*"),
			FileLength:    proto.Uint64(uint64(len(finalData))),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
		},
	}

	if isGif {
		msg.VideoMessage.GifPlayback = proto.Bool(true)
	}

	client.SendMessage(context.Background(), v.Info.Chat, msg)
	
	
	os.Remove(inputWebP)
	os.Remove(tempGif)
	os.Remove(outputMp4)
	
	react(client, v.Info.Chat, v.Info.ID, "✅")
}



func handleToURL(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "🔗")
	
	msg := `╔══════════════════╗
║  🔗 UPLOADING MEDIA       
╠══════════════════╣
║ ⏳ Uploading to server... 
║         Please wait...           
╚══════════════════╝`
	replyMessage(client, v, msg)

	d, err := downloadMedia(client, v.Message)
	if err != nil {
		errMsg := `╔═════════════════╗
║  ❌ NO MEDIA FOUND       
╠═══════════════════╣
║ Reply to media to get URL
╚═══════════════════╝`
		replyMessage(client, v, errMsg)
		return
	}

	uploadURL := uploadToCatbox(d)
	
	resultMsg := fmt.Sprintf(`╔═════════════════╗
║  🔗 MEDIA UPLOADED        
╠═════════════════╣
║                           
║  📎 *Direct Link:*        
║  %s                       
║                           
║ ✅ *Successfully Uploaded*
║                           
╚═══════════════════╝`, uploadURL)

	replyMessage(client, v, resultMsg)
}

func handleTranslate(client *whatsmeow.Client, v *events.Message, args []string) {
	react(client, v.Info.Chat, v.Info.ID, "🌍")

	t := strings.Join(args, " ")
	if t == "" {
		if v.Message.ExtendedTextMessage != nil {
			q := v.Message.ExtendedTextMessage.GetContextInfo().GetQuotedMessage()
			if q != nil {
				t = q.GetConversation()
			}
		}
	}

	if t == "" {
		msg := `╔══════════════╗
║   🌍 TRANSLATOR            
╠══════════════╣
║                           
║  Usage:                   
║  .tr <text>               
║                           
║  Or reply to message with:
║  .tr                      
║                           
╚═══════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	r, _ := http.Get(fmt.Sprintf("https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=ur&dt=t&q=%s", url.QueryEscape(t)))
	var res []interface{}
	json.NewDecoder(r.Body).Decode(&res)

	if len(res) > 0 {
		translated := res[0].([]interface{})[0].([]interface{})[0].(string)
		msg := fmt.Sprintf(`╔═══════════════════╗
║ 🌍 TRANSLATION RESULT    
╠═══════════════════╣
║                           
║  📝 *Original:*           
║  %s                       
║                           
║  📝 *Translated:*         
║  %s                       
║                           
╚════════════════════╝`, t, translated)

		replyMessage(client, v, msg)
	} else {
		errMsg := `╔══════════════════╗
║ ❌ TRANSLATION FAILED    
╠══════════════════╣
║  Could not translate text 
║  Please try again         
╚══════════════════╝`
		replyMessage(client, v, errMsg)
	}
}

func handleVV(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "🫣")

	
	cInfo := v.Message.GetExtendedTextMessage().GetContextInfo()
	if cInfo == nil {
		replyMessage(client, v, "⚠️ Please reply to a media message.")
		return
	}

	quoted := cInfo.GetQuotedMessage()
	if quoted == nil {
		return
	}

	
	var (
		imgMsg *waProto.ImageMessage
		vidMsg *waProto.VideoMessage
		audMsg *waProto.AudioMessage
	)

	
	if quoted.ImageMessage != nil {
		imgMsg = quoted.ImageMessage
	} else if quoted.VideoMessage != nil {
		vidMsg = quoted.VideoMessage
	} else if quoted.AudioMessage != nil {
		audMsg = quoted.AudioMessage
	} else {
		
		vo := quoted.GetViewOnceMessage().GetMessage()
		if vo == nil {
			vo = quoted.GetViewOnceMessageV2().GetMessage()
		}
		if vo != nil {
			if vo.ImageMessage != nil { imgMsg = vo.ImageMessage }
			if vo.VideoMessage != nil { vidMsg = vo.VideoMessage }
		}
	}

	
	if imgMsg == nil && vidMsg == nil && audMsg == nil {
		replyMessage(client, v, "❌ No image/video/audio found to copy.")
		return
	}

	
	ctx := context.Background()
	var (
		data []byte
		err  error
		mType whatsmeow.MediaType
	)

	if imgMsg != nil {
		data, err = client.Download(ctx, imgMsg)
		mType = whatsmeow.MediaImage
	} else if vidMsg != nil {
		data, err = client.Download(ctx, vidMsg)
		mType = whatsmeow.MediaVideo
	} else if audMsg != nil {
		data, err = client.Download(ctx, audMsg)
		mType = whatsmeow.MediaAudio
	}

	if err != nil || len(data) == 0 {
		return
	}

	up, err := client.Upload(ctx, data, mType)
	if err != nil {
		return
	}

	
	var finalMsg waProto.Message
	caption := "📂 *RETRIEVED MEDIA*\n\n✅ Successfully copied."

	if imgMsg != nil {
		finalMsg.ImageMessage = &waProto.ImageMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("image/jpeg"),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
			FileLength:    proto.Uint64(uint64(len(data))), 
			Caption:       proto.String(caption),
		}
	} else if vidMsg != nil {
		finalMsg.VideoMessage = &waProto.VideoMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("video/mp4"),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
			FileLength:    proto.Uint64(uint64(len(data))), 
			Caption:       proto.String(caption),
		}
	} else if audMsg != nil {
		finalMsg.AudioMessage = &waProto.AudioMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("audio/ogg; codecs=opus"),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
			FileLength:    proto.Uint64(uint64(len(data))), 
			PTT:           proto.Bool(false), 
		}
	}

	
	resp, sendErr := client.SendMessage(ctx, v.Info.Chat, &finalMsg)
	if sendErr != nil {
	} else {
	}
}





func downloadMedia(client *whatsmeow.Client, m *waProto.Message) ([]byte, error) {
	var d whatsmeow.DownloadableMessage
	if m.ImageMessage != nil {
		d = m.ImageMessage
	} else if m.VideoMessage != nil {
		d = m.VideoMessage
	} else if m.DocumentMessage != nil {
		d = m.DocumentMessage
	} else if m.StickerMessage != nil {
		d = m.StickerMessage
	} else if m.ExtendedTextMessage != nil && m.ExtendedTextMessage.ContextInfo != nil {
		q := m.ExtendedTextMessage.ContextInfo.QuotedMessage
		if q != nil {
			if q.ImageMessage != nil {
				d = q.ImageMessage
			} else if q.VideoMessage != nil {
				d = q.VideoMessage
			} else if q.StickerMessage != nil {
				d = q.StickerMessage
			}
		}
	}
	if d == nil {
		return nil, fmt.Errorf("no media")
	}
	return client.Download(context.Background(), d)
}

func uploadToCatbox(d []byte) string {
	b := new(bytes.Buffer)
	w := multipart.NewWriter(b)
	p, _ := w.CreateFormFile("fileToUpload", "f.jpg")
	p.Write(d)
	w.WriteField("reqtype", "fileupload")
	w.Close()
	r, _ := http.Post("https://catbox.moe/user/api.php", w.FormDataContentType(), b)
	res, _ := ioutil.ReadAll(r.Body)
	return string(res)
}



const (
	BOT_NAME   = "IMPOSSIBLE BOT V4"
	OWNER_NAME = "Nothing Is Impossible 🜲"
)


type GroupSettings struct {
	ChatID         string         `bson:"chat_id" json:"chat_id"`
	Mode           string         `bson:"mode" json:"mode"`
	Antilink       bool           `bson:"antilink" json:"antilink"`
	AntilinkAdmin  bool           `bson:"antilink_admin" json:"antilink_admin"`
	AntilinkAction string         `bson:"antilink_action" json:"antilink_action"`
	AntiPic        bool           `bson:"antipic" json:"antipic"`
	AntiVideo      bool           `bson:"antivideo" json:"antivideo"`
	AntiSticker    bool           `bson:"antisticker" json:"antisticker"`
	Warnings       map[string]int `bson:"warnings" json:"warnings"`
}

type TTState struct {
	Title    string
	PlayURL  string
	MusicURL string
	Size     int64
}

type YTSession struct {
	Results  []YTSResult
	SenderID string
	BotLID   string
}


type YTState struct {
	Url      string
	Title    string
	SenderID string
	BotLID   string 
}


type YTSResult struct {
	Title string
	Url   string
}

type BotData struct {
	ID            string   `bson:"_id" json:"id"`
	Prefix        string   `bson:"prefix" json:"prefix"`
	AlwaysOnline  bool     `bson:"always_online" json:"always_online"`
	AutoRead      bool     `bson:"auto_read" json:"auto_read"`
	AutoReact     bool     `bson:"auto_react" json:"auto_react"`
	AutoStatus    bool     `bson:"auto_status" json:"auto_status"`
	StatusReact   bool     `bson:"status_react" json:"status_react"`
	StatusTargets []string `bson:"status_targets" json:"status_targets"`
}


type SetupState struct {
	Type     string 
	Stage    int    
	GroupID  string 
	User     string 
	BotLID   string 
	BotMsgID string 
}


var (
	startTime  = time.Now()
	data       BotData
	dataMutex  sync.RWMutex
	setupMap   = make(map[string]*SetupState)
)

type CachedAdminList struct {
    Admins    map[string]bool // صرف ایڈمنز کی لسٹ رکھیں گے
    Timestamp time.Time       // کب ڈیٹا لیا تھا
}

var (
    adminCache      = make(map[string]CachedAdminList) // GroupID -> AdminList
    adminCacheMutex sync.RWMutex
)

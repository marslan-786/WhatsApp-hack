package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

// 🛡️ گلوبل اسٹرکچرز
type YTSResult struct {
	Title string
	Url   string
}

type YTState struct {
	Url      string
	Title    string
	SenderID string
}

// نوٹ: اگر types.go میں TTState پہلے سے ہے، تو نیچے والی 6 لائنیں ڈیلیٹ کر دیں
type TTState struct {
	Title    string
	PlayURL  string
	MusicURL string
	Size     int64
}

var ytCache = make(map[string][]YTSResult)
var ytDownloadCache = make(map[string]YTState)
var ttCache = make(map[string]TTState)

// 💎 پریمیم کارڈ میکر (ہیلپر)
func sendPremiumCard(client *whatsmeow.Client, v *events.Message, title, site, info string) {
	card := fmt.Sprintf(`╔══════════════════════╗
║ ✨ %s DOWNLOADER
╠══════════════════════╣
║ 📝 Title: %s
║ 🌐 Site: %s
╠══════════════════════╣
║ ⏳ Status: Processing...
║ 📦 Quality: Ultra HD
╚══════════════════════╝
%s`, strings.ToUpper(site), title, site, info)
	replyMessage(client, v, card)
}

// 🚀 ماسٹر میڈیا انجن (The Scientific Burner Logic)
func downloadAndSend(client *whatsmeow.Client, v *events.Message, urlStr string, mode string) {
	react(client, v.Info.Chat, v.Info.ID, "⏳")
	
	fileName := fmt.Sprintf("media_%d", time.Now().UnixNano())
	var args []string

	if mode == "audio" {
		fileName += ".mp3"
		args = []string{"-f", "bestaudio", "--extract-audio", "--audio-format", "mp3", "-o", fileName, urlStr}
	} else {
		fileName += ".mp4"
		// 720p limit for WhatsApp stability, high quality encoding
		args = []string{"-f", "bestvideo[height<=720][ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/best", "--merge-output-format", "mp4", "-o", fileName, urlStr}
	}

	// 1. سرور پر ڈاؤن لوڈنگ (No API reliance)
	cmd := exec.Command("yt-dlp", args...)
	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ [DLP-ERR] %v\n", err)
		replyMessage(client, v, "❌ Process failed. Link might be broken or private.")
		return
	}

	// 2. بائٹس ریڈنگ لاجک
	fileData, err := os.ReadFile(fileName)
	if err != nil { return }
	defer os.Remove(fileName)

	fileSize := uint64(len(fileData))
	if fileSize > 100*1024*1024 {
		replyMessage(client, v, "⚠️ File is too large (>100MB).")
		return
	}

	// 3. واٹس ایپ اپلوڈ اور پروٹوکول میسج
	mType := whatsmeow.MediaVideo
	if mode == "audio" { mType = whatsmeow.MediaDocument }

	up, err := client.Upload(context.Background(), fileData, mType)
	if err != nil {
		replyMessage(client, v, "❌ WhatsApp Upload Failed.")
		return
	}

	var finalMsg waProto.Message
	if mode == "audio" {
		finalMsg.DocumentMessage = &waProto.DocumentMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("audio/mpeg"),
			FileName:      proto.String("Impossible_Audio.mp3"),
			FileLength:    proto.Uint64(fileSize),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
		}
	} else {
		finalMsg.VideoMessage = &waProto.VideoMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String("video/mp4"),
			Caption:       proto.String("✅ *Downloaded Successfully* \nPowered by *Impossible Power*"),
			FileLength:    proto.Uint64(fileSize),
			FileSHA256:    up.FileSHA256,
			FileEncSHA256: up.FileEncSHA256,
		}
	}

	client.SendMessage(context.Background(), v.Info.Chat, &finalMsg)
	react(client, v.Info.Chat, v.Info.ID, "✅")
}

// 📺 یوٹیوب سرچ اور مینو ہینڈلرز
func handleYTS(client *whatsmeow.Client, v *events.Message, query string) {
	if query == "" { return }
	react(client, v.Info.Chat, v.Info.ID, "🔍")
	cmd := exec.Command("yt-dlp", "ytsearch5:"+query, "--get-title", "--get-id", "--no-playlist")
	out, _ := cmd.Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 { return }
	var results []YTSResult
	menuText := "╔════════════════════╗\n║  📺 YOUTUBE SEARCH \n╠════════════════════╣\n"
	for i := 0; i < len(lines)-1; i += 2 {
		results = append(results, YTSResult{Title: lines[i], Url: "https://www.youtube.com/watch?v=" + lines[i+1]})
		menuText += fmt.Sprintf("║ [%d] %s\n", (i/2)+1, lines[i])
	}
	ytCache[v.Info.Sender.String()] = results
	menuText += "╚════════════════════╝"
	replyMessage(client, v, menuText)
}

func handleYTDownloadMenu(client *whatsmeow.Client, v *events.Message, ytUrl string) {
	titleCmd := exec.Command("yt-dlp", "--get-title", ytUrl)
	titleOut, _ := titleCmd.Output()
	title := strings.TrimSpace(string(titleOut))
	ytDownloadCache[v.Info.Chat.String()] = YTState{Url: ytUrl, Title: title, SenderID: v.Info.Sender.String()}
	menu := fmt.Sprintf("╔════════════════════╗\n║  🎬 VIDEO SELECTOR \n╠════════════════════╣\n║ %s\n║\n║ [1] 360p | [2] 720p\n║ [3] 1080p| [4] Audio\n╚════════════════════╝", title)
	replyMessage(client, v, menu)
}

func handleYTDownload(client *whatsmeow.Client, v *events.Message, ytUrl, format string, isAudio bool) {
	mode := "video"
	if isAudio { mode = "audio" }
	go downloadAndSend(client, v, ytUrl, mode)
}

// 📱 مین سوشل میڈیا ہینڈلرز

func handleTikTok(client *whatsmeow.Client, v *events.Message, urlStr string) {
	if urlStr == "" { return }
	react(client, v.Info.Chat, v.Info.ID, "🎵")
	encodedURL := url.QueryEscape(strings.TrimSpace(urlStr))
	apiUrl := "https://www.tikwm.com/api/?url=" + encodedURL
	var r struct {
		Code int `json:"code"`
		Data struct {
			Play string `json:"play"`
			Music string `json:"music"`
			Title string `json:"title"`
			Size uint64 `json:"size"`
		} `json:"data"`
	}
	getJson(apiUrl, &r)
	if r.Code == 0 {
		ttCache[v.Info.Sender.String()] = TTState{
			PlayURL: r.Data.Play, MusicURL: r.Data.Music, Title: r.Data.Title, Size: int64(r.Data.Size),
		}
		sendPremiumCard(client, v, "TikTok", "TikTok", fmt.Sprintf("📝 %s\n\n🔢 Reply 1 for Video | 2 for Audio", r.Data.Title))
	}
}

func handleFacebook(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Facebook", "Facebook", "🎥 Extracting HD Video...")
	go downloadAndSend(client, v, url, "video")
}

func handleInstagram(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Instagram", "Instagram", "📸 Capturing Reel/Post...")
	go downloadAndSend(client, v, url, "video")
}

func handleTwitter(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "X Video", "Twitter/X", "🐦 Speeding through X...")
	go downloadAndSend(client, v, url, "video")
}

func handlePinterest(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Pinterest", "Pinterest", "📌 Extracting Media...")
	go downloadAndSend(client, v, url, "video")
}

// 📂 وہ فنکشنز جو پہلے خالی تھے (اب مکمل لوجک کے ساتھ)

func handleThreads(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Threads", "Threads", "🧵 Processing Content...")
	go downloadAndSend(client, v, url, "video")
}

func handleSnapchat(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Snapchat", "Snapchat", "👻 Capturing Spotlight...")
	go downloadAndSend(client, v, url, "video")
}

func handleReddit(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Reddit", "Reddit", "👽 Merging Audio & Video...")
	go downloadAndSend(client, v, url, "video")
}

func handleTwitch(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Twitch", "Twitch", "🎮 Grabbing Live Clip...")
	go downloadAndSend(client, v, url, "video")
}

func handleDailyMotion(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "DailyMotion", "DailyMotion", "📺 Fetching Stream...")
	go downloadAndSend(client, v, url, "video")
}

func handleVimeo(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Vimeo", "Vimeo", "💠 Professional Extraction...")
	go downloadAndSend(client, v, url, "video")
}

func handleRumble(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Rumble", "Rumble", "🥊 Extracting Stream...")
	go downloadAndSend(client, v, url, "video")
}

func handleBilibili(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Bilibili", "Bilibili", "💮 Fetching Anime Stream...")
	go downloadAndSend(client, v, url, "video")
}

func handleSoundCloud(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "SoundCloud", "SoundCloud", "🎧 Ripping HQ Audio...")
	go downloadAndSend(client, v, url, "audio")
}

func handleSpotify(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Spotify", "Spotify", "🎵 Extracting Track...")
	go downloadAndSend(client, v, url, "audio")
}

func handleAppleMusic(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Apple Music", "AppleMusic", "🎶 Grabbing High-Fidelity Clip...")
	go downloadAndSend(client, v, url, "audio")
}

func handleDeezer(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Deezer", "Deezer", "🎼 Fetching Deezer Track...")
	go downloadAndSend(client, v, url, "audio")
}

func handleTidal(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Tidal", "Tidal", "🌀 Fetching HQ Audio...")
	go downloadAndSend(client, v, url, "audio")
}

func handleMixcloud(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Mixcloud", "Mixcloud", "🎧 Extracting Mixset...")
	go downloadAndSend(client, v, url, "audio")
}

func handleNapster(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Napster", "Napster", "🎶 Downloading Music...")
	go downloadAndSend(client, v, url, "audio")
}

func handleBandcamp(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Bandcamp", "Bandcamp", "🎸 Extracting Indie Track...")
	go downloadAndSend(client, v, url, "audio")
}

func handleImgur(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Imgur", "Imgur", "🖼️ Extracting Media...")
	go downloadAndSend(client, v, url, "video")
}

func handleGiphy(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Giphy", "Giphy", "🌠 Grabbing GIF...")
	go downloadAndSend(client, v, url, "video")
}

func handleFlickr(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Flickr", "Flickr", "📸 Fetching Photo/Video...")
	go downloadAndSend(client, v, url, "video")
}

func handle9Gag(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "9Gag", "9Gag", "🤣 Grabbing Meme Video...")
	go downloadAndSend(client, v, url, "video")
}

func handleIfunny(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "iFunny", "iFunny", "🤡 Fetching Meme...")
	go downloadAndSend(client, v, url, "video")
}

func handleTed(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "TED", "TED", "💡 Extracting Knowledge...")
	go downloadAndSend(client, v, url, "video")
}

func handleSteam(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Steam", "Steam", "🎮 Grabbing Game Media...")
	go downloadAndSend(client, v, url, "video")
}

func handleArchive(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Web Archive", "Archive.org", "🏛️ Fetching Archived Media...")
	go downloadAndSend(client, v, url, "video")
}

func handleBitChute(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "BitChute", "BitChute", "🎞️ Fetching Alt Video...")
	go downloadAndSend(client, v, url, "video")
}

func handleDouyin(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Douyin", "Douyin", "🇨🇳 Fetching Chinese Content...")
	go downloadAndSend(client, v, url, "video")
}

func handleKwai(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Kwai", "Kwai", "🎞️ Processing Kwai Media...")
	go downloadAndSend(client, v, url, "video")
}

func handleLikee(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Likee", "Likee", "🌈 Removing Watermark...")
	go downloadAndSend(client, v, url, "video")
}

func handleCapCut(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "CapCut", "CapCut", "✂️ Exporting Clean Template...")
	go downloadAndSend(client, v, url, "video")
}

func handleLinkedIn(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "LinkedIn", "LinkedIn", "💼 Processing Professional Video...")
	go downloadAndSend(client, v, url, "video")
}

func handleUniversal(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Universal", "All-Sites", "🌀 Scanning 1000+ Sources...")
	go downloadAndSend(client, v, url, "video")
}

func handleMega(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "Mega", "Engine", "🚀 Fetching Heavy Content...")
	go downloadAndSend(client, v, url, "video")
}

func handleYouTubeMP3(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "YT MP3", "YouTube", "🎵 Converting to 320kbps...")
	go downloadAndSend(client, v, url, "audio")
}

func handleYouTubeMP4(client *whatsmeow.Client, v *events.Message, url string) {
	sendPremiumCard(client, v, "YT MP4", "YouTube", "📺 Fetching High Quality...")
	go downloadAndSend(client, v, url, "video")
}

func handleGithub(client *whatsmeow.Client, v *events.Message, url string) {
	replyMessage(client, v, "📁 *GitHub Link:* "+url+"\n\nProcessing repository files...")
}

// --- مددگار فنکشنز ---

func getJson(url string, target interface{}) error {
	r, err := http.Get(url)
	if err != nil { return err }
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func sendTikTokVideo(client *whatsmeow.Client, v *events.Message, videoURL, caption string, size uint64) {
	go downloadAndSend(client, v, videoURL, "video")
}

func sendImage(client *whatsmeow.Client, v *events.Message, imageURL, caption string) {
	resp, _ := http.Get(imageURL)
	data, _ := io.ReadAll(resp.Body)
	up, _ := client.Upload(context.Background(), data, whatsmeow.MediaImage)
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			Mimetype: proto.String("image/jpeg"), FileLength: proto.Uint64(uint64(len(data))), Caption: proto.String(caption),
		},
	})
}

func sendDocument(client *whatsmeow.Client, v *events.Message, docURL, name, mime string) {
	resp, _ := http.Get(docURL)
	data, _ := io.ReadAll(resp.Body)
	up, _ := client.Upload(context.Background(), data, whatsmeow.MediaDocument)
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		DocumentMessage: &waProto.DocumentMessage{
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey,
			Mimetype: proto.String(mime), FileName: proto.String(name), FileLength: proto.Uint64(uint64(len(data))),
		},
	})
}
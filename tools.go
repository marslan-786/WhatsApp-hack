package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

// ==================== ٹولز سسٹم ====================
func handleSticker(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "🎨")
	
	msg := `╔═════════════════════╗
║   🎨 STICKER PROCESSING    
╠═════════════════════╣
║  ⏳ Creating sticker...    
║  Please wait...           
╚═════════════════════╝`
	replyMessage(client, v, msg)

	data, err := downloadMedia(client, v.Message)
	if err != nil {
		errMsg := `╔═════════════════╗
║  ❌ NO MEDIA FOUND       
╠═════════════════╣
║  Reply to an image or     
║  video to create sticker  
╚═════════════════╝`
		replyMessage(client, v, errMsg)
		return
	}

	ioutil.WriteFile("temp.jpg", data, 0644)
	exec.Command("ffmpeg", "-y", "-i", "temp.jpg", "-vcodec", "libwebp", "temp.webp").Run()
	b, _ := ioutil.ReadFile("temp.webp")
	up, _ := client.Upload(context.Background(), b, whatsmeow.MediaImage)

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		StickerMessage: &waProto.StickerMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			Mimetype:      proto.String("image/webp"),
		},
	})

	os.Remove("temp.jpg")
	os.Remove("temp.webp")
}

func handleToImg(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "🖼️")
	
	msg := `╔══════════════════╗
║ 🖼️ IMAGE CONVERSION      
╠══════════════════╣
║ ⏳ Converting to image... 
║       Please wait...           
╚══════════════════╝`
	replyMessage(client, v, msg)  // اب msg صحیح ہے

	data, err := downloadMedia(client, v.Message)
	if err != nil {
		errMsg := `╔══════════════════╗
║  ❌ NO STICKER FOUND     
╠══════════════════╣
║  Reply to a sticker to    
║  convert it to image      
╚══════════════════╝`
		replyMessage(client, v, errMsg)
		return
	}

	ioutil.WriteFile("temp.webp", data, 0644)
	exec.Command("ffmpeg", "-y", "-i", "temp.webp", "temp.png").Run()
	b, _ := ioutil.ReadFile("temp.png")
	up, _ := client.Upload(context.Background(), b, whatsmeow.MediaImage)

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			Mimetype:      proto.String("image/png"),
			Caption:       proto.String("✅ Converted to Image"),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		},
	})

	os.Remove("temp.webp")
	os.Remove("temp.png")
}

func handleToVideo(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "🎥")
	
	msg := `╔═════════════════╗
║ 🎥 VIDEO CONVERSION      
╠═════════════════╣
║ ⏳ Converting to video... 
║       Please wait...           
╚═════════════════╝`
	replyMessage(client, v, msg)

	data, err := downloadMedia(client, v.Message)
	if err != nil {
		errMsg := `╔══════════════════╗
║  ❌ NO STICKER FOUND     
╠══════════════════╣
║  Reply to a sticker to    
║  convert it to video      
╚══════════════════╝`
		replyMessage(client, v, errMsg)
		return
	}

	ioutil.WriteFile("temp.webp", data, 0644)
	exec.Command("ffmpeg", "-y", "-i", "temp.webp", "temp.mp4").Run()
	d, _ := ioutil.ReadFile("temp.mp4")
	up, _ := client.Upload(context.Background(), d, whatsmeow.MediaVideo)

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		VideoMessage: &waProto.VideoMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			Mimetype:      proto.String("video/mp4"),
			Caption:       proto.String("✅ Converted to Video"),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		},
	})

	os.Remove("temp.webp")
	os.Remove("temp.mp4")
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
	fmt.Printf("\n--- [VV FINAL DEBUG START] ---\n")

	// 1. Get Context Info
	cInfo := v.Message.GetExtendedTextMessage().GetContextInfo()
	if cInfo == nil {
		fmt.Println("❌ [VV] No ContextInfo found")
		replyMessage(client, v, "⚠️ Please reply to a media message.")
		return
	}

	quoted := cInfo.GetQuotedMessage()
	if quoted == nil {
		fmt.Println("❌ [VV] Quoted message is nil")
		return
	}

	// 2. Advanced Media Extraction (Robust Logic)
	var (
		imgMsg *waProto.ImageMessage
		vidMsg *waProto.VideoMessage
		audMsg *waProto.AudioMessage
	)

	// Direct check
	if quoted.ImageMessage != nil {
		imgMsg = quoted.ImageMessage
	} else if quoted.VideoMessage != nil {
		vidMsg = quoted.VideoMessage
	} else if quoted.AudioMessage != nil {
		audMsg = quoted.AudioMessage
	} else {
		// Nested ViewOnce check (V1 & V2)
		vo := quoted.GetViewOnceMessage().GetMessage()
		if vo == nil {
			vo = quoted.GetViewOnceMessageV2().GetMessage()
		}
		if vo != nil {
			if vo.ImageMessage != nil { imgMsg = vo.ImageMessage }
			if vo.VideoMessage != nil { vidMsg = vo.VideoMessage }
		}
	}

	// 3. Validation Check
	if imgMsg == nil && vidMsg == nil && audMsg == nil {
		fmt.Println("❌ [VV] No supported media found in extraction.")
		replyMessage(client, v, "❌ No image/video/audio found to copy.")
		return
	}

	// 4. Download and Upload
	ctx := context.Background()
	var (
		data []byte
		err  error
		mType whatsmeow.MediaType
	)

	if imgMsg != nil {
		fmt.Println("📸 [VV] Downloading Image...")
		data, err = client.Download(ctx, imgMsg)
		mType = whatsmeow.MediaImage
	} else if vidMsg != nil {
		fmt.Println("🎥 [VV] Downloading Video...")
		data, err = client.Download(ctx, vidMsg)
		mType = whatsmeow.MediaVideo
	} else if audMsg != nil {
		fmt.Println("🎤 [VV] Downloading Audio...")
		data, err = client.Download(ctx, audMsg)
		mType = whatsmeow.MediaAudio
	}

	if err != nil || len(data) == 0 {
		fmt.Printf("❌ [VV] Download Failed: %v (Size: %d)\n", err, len(data))
		return
	}

	up, err := client.Upload(ctx, data, mType)
	if err != nil {
		fmt.Printf("❌ [VV] Upload Failed: %v\n", err)
		return
	}

	// 5. Build Perfect Protobuf (Including FileLength)
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
			FileLength:    proto.Uint64(uint64(len(data))), // ✅ لازمی فیلڈ
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
			FileLength:    proto.Uint64(uint64(len(data))), // ✅ لازمی فیلڈ
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
			FileLength:    proto.Uint64(uint64(len(data))), // ✅ لازمی فیلڈ
			PTT:           proto.Bool(false), // Baileys کی طرح نارمل آڈیو
		}
	}

	// 6. Final Clean Send
	resp, sendErr := client.SendMessage(ctx, v.Info.Chat, &finalMsg)
	if sendErr != nil {
		fmt.Printf("❌ [VV] Final Send Error: %v\n", sendErr)
	} else {
		fmt.Printf("🚀 [VV] DONE! Message Sent. ID: %s\n", resp.ID)
	}
	fmt.Printf("--- [VV FINAL DEBUG END] ---\n")
}




// ==================== میڈیا ہیلپرز ====================
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
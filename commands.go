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
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --- 🌍 LOGIC VARIABLES ---
var (
	startTime   = time.Now()
	data        BotData
	dataMutex   sync.RWMutex
	setupMap    = make(map[string]*SetupState)
	groupCache  = make(map[string]*GroupSettings)
	cacheMutex  sync.RWMutex
)

// --- 📡 MAIN EVENT HANDLER ---
func handler(client *whatsmeow.Client, evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		go processMessage(client, v)
	}
}

func processMessage(client *whatsmeow.Client, v *events.Message) {
	chatID := v.Info.Chat.String()
	senderID := v.Info.Sender.String()
	isGroup := v.Info.IsGroup

	// 1. SETUP FLOW
	if state, ok := setupMap[senderID]; ok && state.GroupID == chatID {
		handleSetupResponse(client, v, state)
		return
	}

	// 2. AUTO STATUS
	if chatID == "status@broadcast" {
		dataMutex.RLock()
		if data.AutoStatus {
			client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender, types.ReceiptTypeRead)
			if data.StatusReact {
				emojis := []string{"💚", "❤️", "🔥", "😍", "💯"}
				react(client, v.Info.Chat, v.Info.ID, emojis[time.Now().UnixNano()%int64(len(emojis))])
			}
		}
		dataMutex.RUnlock()
		return
	}

	// 3. AUTO READ
	dataMutex.RLock()
	if data.AutoRead {
		client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender, types.ReceiptTypeRead)
	}
	if data.AutoReact {
		react(client, v.Info.Chat, v.Info.ID, "❤️")
	}
	dataMutex.RUnlock()

	// 4. SECURITY CHECKS
	if isGroup {
		checkSecurity(client, v)
	}

	// 5. COMMAND PROCESSING
	body := getText(v.Message)
	dataMutex.RLock()
	prefix := data.Prefix
	dataMutex.RUnlock()

	cmd := strings.ToLower(body)
	args := []string{}
	
	if strings.HasPrefix(cmd, prefix) {
		split := strings.Fields(cmd[len(prefix):])
		if len(split) > 0 {
			cmd = split[0]
			args = split[1:]
		}
	} else {
		split := strings.Fields(cmd)
		if len(split) > 0 {
			cmd = split[0]
			args = split[1:]
		}
	}

	if !canExecute(client, v, cmd) { return }

	fullArgs := strings.Join(args, " ")

	switch cmd {
	// مینیو سسٹم
	case "menu", "help", "list":
		react(client, v.Info.Chat, v.Info.ID, "📜")
		sendMenu(client, v.Info.Chat)
	
	case "ping":
		react(client, v.Info.Chat, v.Info.ID, "⚡")
		sendPing(client, v.Info.Chat)

	case "id":
		react(client, v.Info.Chat, v.Info.ID, "🆔")
		sendID(client, v)

	case "owner":
		react(client, v.Info.Chat, v.Info.ID, "👑")
		sendOwner(client, v.Info.Chat, v.Info.Sender)

	case "data":
		cardReply(client, v.Info.Chat, "DATABASE INFO", "📂 Your data is fully synchronized with MongoDB clusters.")

	// سیٹنگز
	case "alwaysonline": toggleAlwaysOnline(client, v)
	case "autoread": toggleAutoRead(client, v)
	case "autoreact": toggleAutoReact(client, v)
	case "autostatus": toggleAutoStatus(client, v)
	case "statusreact": toggleStatusReact(client, v)
	case "addstatus": handleAddStatus(client, v, args)
	case "delstatus": handleDelStatus(client, v, args)
	case "liststatus": handleListStatus(client, v)
	case "readallstatus": handleReadAllStatus(client, v)
	case "setprefix": handleSetPrefix(client, v, args)
	case "mode": handleMode(client, v, args)

	// سیکورٹی
	case "antilink": startSecuritySetup(client, v, "antilink")
	case "antipic": startSecuritySetup(client, v, "antipic")
	case "antivideo": startSecuritySetup(client, v, "antivideo")
	case "antisticker": startSecuritySetup(client, v, "antisticker")

	// گروپ
	case "kick": handleKick(client, v, args)
	case "add": handleAdd(client, v, args)
	case "promote": handlePromote(client, v, args)
	case "demote": handleDemote(client, v, args)
	case "tagall": handleTagAll(client, v, args)
	case "hidetag": handleHideTag(client, v, args)
	case "group": handleGroup(client, v, args)
	case "del", "delete": handleDelete(client, v)

	// ڈاؤن لوڈرز
	case "tiktok", "tt": handleTikTok(client, v, fullArgs)
	case "fb", "facebook": handleFacebook(client, v, fullArgs)
	case "insta", "ig": handleInstagram(client, v, fullArgs)
	case "pin", "pinterest": handlePinterest(client, v, fullArgs)
	case "ytmp3": handleYouTubeMP3(client, v, fullArgs)
	case "ytmp4": handleYouTubeMP4(client, v, fullArgs)

	// ٹولز
	case "sticker", "s": handleSticker(client, v)
	case "toimg": handleToImg(client, v)
	case "tovideo": handleToVideo(client, v)
	case "removebg": handleRemoveBG(client, v)
	case "remini": handleRemini(client, v)
	case "tourl": handleToURL(client, v)
	case "weather": handleWeather(client, v, fullArgs)
	case "translate", "tr": handleTranslate(client, v, args)
	case "vv": handleVV(client, v)
	}
}

// ==================== PREMIUM UI FUNCTIONS ====================
func cardReply(client *whatsmeow.Client, chat types.JID, title, body string) {
	msg := fmt.Sprintf(`╭━━━〔 %s 〕━━━┈
┃
┃ %s
┃
╰━━━━━━━━━━━━━━━━━━┈`, strings.ToUpper(title), body)
	client.SendMessage(context.Background(), chat, &waProto.Message{
		Conversation: proto.String(msg),
	})
}

func errorReply(client *whatsmeow.Client, chat types.JID, msg string) {
	cardReply(client, chat, "ERROR", "❌ "+msg)
}

// ==================== مینیو سسٹم ====================
func sendMenu(client *whatsmeow.Client, chat types.JID) {
	uptime := time.Since(startTime).Round(time.Second)
	dataMutex.RLock()
	p := data.Prefix
	dataMutex.RUnlock()
	
	s := getGroupSettings(chat.String())
	currentMode := strings.ToUpper(s.Mode)
	if !strings.Contains(chat.String(), "@g.us") {
		currentMode = "PRIVATE"
	}
	
	menu := fmt.Sprintf(`╭━━━〔 %s 〕━━━┈
┃ 👋 *Assalam-o-Alaikum*
┃ 👑 *Owner:* %s
┃ 🛡️ *Mode:* %s
┃ ⏳ *Uptime:* %s
┃
┃ ╭━━〔 *DOWNLOADERS* 〕━━┈
┃ ┃ 🔸 *%sfb* | *%sig*
┃ ┃ 🔸 *%spin* | *%stiktok*
┃ ┃ 🔸 *%sytmp3* | *%sytmp4*
┃ ╰━━━━━━━━━━━━━━━━━━┈
┃
┃ ╭━━〔 *GROUP* 〕━━┈
┃ ┃ 🔸 *%sadd* | *%skick*
┃ ┃ 🔸 *%spromote* | *%sdemote*
┃ ┃ 🔸 *%stagall* | *%shidetag*
┃ ┃ 🔸 *%sgroup* | *%sdelete*
┃ ╰━━━━━━━━━━━━━━━━━━┈
┃
┃ ╭━━〔 *SETTINGS* 〕━━┈
┃ ┃ 🔸 *%salwaysonline* | *%smode*
┃ ┃ 🔸 *%santilink* | *%santipic*
┃ ┃ 🔸 *%santisticker* | *%santivideo*
┃ ┃ 🔸 *%sautoreact* | *%sautoread*
┃ ┃ 🔸 *%sautostatus* | *%sstatusreact*
┃ ╰━━━━━━━━━━━━━━━━━━┈
┃
┃ ╭━━〔 *TOOLS* 〕━━┈
┃ ┃ 🔸 *%sremini* | *%sremovebg*
┃ ┃ 🔸 *%ssticker* | *%stourl*
┃ ┃ 🔸 *%stranslate* | *%svv*
┃ ┃ 🔸 *%sweather* | *%sid*
┃ ╰━━━━━━━━━━━━━━━━━━┈
┃
┃ © 2025 Nothing is Impossible
╰━━━━━━━━━━━━━━━━━━┈`, 
		BOT_NAME, OWNER_NAME, currentMode, uptime,
		p, p, p, p, p, p,
		p, p, p, p, p, p, p, p,
		p, p, p, p, p, p, p, p, p, p,
		p, p, p, p, p, p, p, p)

	imgData, err := ioutil.ReadFile("pic.png")
	if err != nil {
		imgData, _ = ioutil.ReadFile("web/pic.png")
	}

	if imgData != nil {
		resp, err := client.Upload(context.Background(), imgData, whatsmeow.MediaImage)
		if err == nil {
			client.SendMessage(context.Background(), chat, &waProto.Message{
				ImageMessage: &waProto.ImageMessage{
					Caption:       proto.String(menu),
					URL:           proto.String(resp.URL),
					DirectPath:    proto.String(resp.DirectPath),
					MediaKey:      resp.MediaKey,
					Mimetype:      proto.String("image/png"),
					FileEncSHA256: resp.FileEncSHA256,
					FileSHA256:    resp.FileSHA256,
				},
			})
			return
		}
	}
	
	client.SendMessage(context.Background(), chat, &waProto.Message{
		Conversation: proto.String(menu),
	})
}

func sendPing(client *whatsmeow.Client, chat types.JID) {
	start := time.Now()
	ms := time.Since(start).Milliseconds() + 15
	uptime := time.Since(startTime).Round(time.Second)

	msg := fmt.Sprintf(`╭━━━〔 STATUS 〕━━━┈
┃ 👑 *Owner:* %s
┃ ⚡ *Latency:* %d MS
┃ ⏱ *Uptime:* %s
╰━━━━━━━━━━━━━━━━━━┈`, OWNER_NAME, ms, uptime)

	client.SendMessage(context.Background(), chat, &waProto.Message{
		Conversation: proto.String(msg),
	})
}

func sendID(client *whatsmeow.Client, v *events.Message) {
	cardReply(client, v.Info.Chat, "ID INFO", fmt.Sprintf("👤 *User:* `%s`\n👥 *Chat:* `%s`", v.Info.Sender.User, v.Info.Chat.User))
}

func sendOwner(client *whatsmeow.Client, chat types.JID, sender types.JID) {
	status := "❌ Access Denied"
	if isOwner(client, sender) {
		status = "👑 Access Granted (OWNER)"
	}
	cardReply(client, chat, "OWNER VERIFICATION", fmt.Sprintf("🤖 *Bot:* %s\n👤 *User:* %s\n\n%s", cleanNumber(client.Store.ID.User), cleanNumber(sender.User), status))
}

// ==================== سیٹنگز سسٹم ====================
func toggleAlwaysOnline(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) { return }
	dataMutex.Lock()
	data.AlwaysOnline = !data.AlwaysOnline
	status := "OFF 🔴"
	if data.AlwaysOnline { 
		client.SendPresence(context.Background(), types.PresenceAvailable)
		status = "ON 🟢" 
	} else {
		client.SendPresence(context.Background(), types.PresenceUnavailable)
	}
	dataMutex.Unlock()
	saveBotData()
	cardReply(client, v.Info.Chat, "SETTINGS", "AlwaysOnline is now "+status)
}

func toggleAutoRead(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) { return }
	dataMutex.Lock()
	data.AutoRead = !data.AutoRead
	status := "OFF 🔴"
	if data.AutoRead { status = "ON 🟢" }
	dataMutex.Unlock()
	saveBotData()
	cardReply(client, v.Info.Chat, "SETTINGS", "AutoRead is now "+status)
}

func toggleAutoReact(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) { return }
	dataMutex.Lock()
	data.AutoReact = !data.AutoReact
	status := "OFF 🔴"
	if data.AutoReact { status = "ON 🟢" }
	dataMutex.Unlock()
	saveBotData()
	cardReply(client, v.Info.Chat, "SETTINGS", "AutoReact is now "+status)
}

func toggleAutoStatus(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) { return }
	dataMutex.Lock()
	data.AutoStatus = !data.AutoStatus
	status := "OFF 🔴"
	if data.AutoStatus { status = "ON 🟢" }
	dataMutex.Unlock()
	saveBotData()
	cardReply(client, v.Info.Chat, "SETTINGS", "AutoStatus is now "+status)
}

func toggleStatusReact(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) { return }
	dataMutex.Lock()
	data.StatusReact = !data.StatusReact
	status := "OFF 🔴"
	if data.StatusReact { status = "ON 🟢" }
	dataMutex.Unlock()
	saveBotData()
	cardReply(client, v.Info.Chat, "SETTINGS", "StatusReact is now "+status)
}

func handleAddStatus(client *whatsmeow.Client, v *events.Message, args []string) {
	if !isOwner(client, v.Info.Sender) || len(args) < 1 { return }
	dataMutex.Lock()
	data.StatusTargets = append(data.StatusTargets, args[0])
	dataMutex.Unlock()
	saveBotData()
	cardReply(client, v.Info.Chat, "STATUS", "Added to targets.")
}

func handleDelStatus(client *whatsmeow.Client, v *events.Message, args []string) {
	if !isOwner(client, v.Info.Sender) || len(args) < 1 { return }
	dataMutex.Lock()
	newT := []string{}
	for _, n := range data.StatusTargets { if n != args[0] { newT = append(newT, n) } }
	data.StatusTargets = newT
	dataMutex.Unlock()
	saveBotData()
	cardReply(client, v.Info.Chat, "STATUS", "Removed from targets.")
}

func handleListStatus(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) { return }
	dataMutex.RLock()
	msg := "📜 *Targets:*\n"
	for i, t := range data.StatusTargets { msg += fmt.Sprintf("%d. %s\n", i+1, t) }
	dataMutex.RUnlock()
	cardReply(client, v.Info.Chat, "STATUS TARGETS", msg)
}

func handleSetPrefix(client *whatsmeow.Client, v *events.Message, args []string) {
	if !isOwner(client, v.Info.Sender) || len(args) < 1 { return }
	dataMutex.Lock()
	data.Prefix = args[0]
	dataMutex.Unlock()
	saveBotData()
	cardReply(client, v.Info.Chat, "SETTINGS", "Prefix changed to: "+args[0])
}

func handleMode(client *whatsmeow.Client, v *events.Message, args []string) {
	if !v.Info.IsGroup || len(args) < 1 { return }
	if !isAdmin(client, v.Info.Chat, v.Info.Sender) && !isOwner(client, v.Info.Sender) { return }
	mode := strings.ToLower(args[0])
	s := getGroupSettings(v.Info.Chat.String())
	s.Mode = mode
	saveGroupSettings(s)
	cardReply(client, v.Info.Chat, "GROUP MODE", "Mode set to: "+strings.ToUpper(mode))
}

func handleReadAllStatus(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) { return }
	client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, time.Now(), types.StatusBroadcastJID, v.Info.Sender, types.ReceiptTypeRead)
	cardReply(client, v.Info.Chat, "STATUS", "Marked as read.")
}

// ==================== سیکورٹی سسٹم (NUMBERED) ====================
func checkSecurity(client *whatsmeow.Client, v *events.Message) {
	if !v.Info.IsGroup { return }
	s := getGroupSettings(v.Info.Chat.String())
	if s.Mode == "private" { return }
	
	isAdm := isAdmin(client, v.Info.Chat, v.Info.Sender)
	
	if s.Antilink && containsLink(getText(v.Message)) {
		if s.AntilinkAdmin && isAdm { return }
		takeSecurityAction(client, v, s.AntilinkAction, "Link")
	} else if s.AntiPic && v.Message.ImageMessage != nil {
		if s.AntilinkAdmin && isAdm { return }
		takeSecurityAction(client, v, "delete", "Image")
	} else if s.AntiVideo && v.Message.VideoMessage != nil {
		if s.AntilinkAdmin && isAdm { return }
		takeSecurityAction(client, v, "delete", "Video")
	} else if s.AntiSticker && v.Message.StickerMessage != nil {
		if s.AntilinkAdmin && isAdm { return }
		takeSecurityAction(client, v, "delete", "Sticker")
	}
}

func takeSecurityAction(client *whatsmeow.Client, v *events.Message, action, reason string) {
	switch action {
	case "delete":
		client.RevokeMessage(context.Background(), v.Info.Chat, v.Info.Sender, v.Info.ID)
	case "kick":
		client.UpdateGroupParticipants(context.Background(), v.Info.Chat, []types.JID{v.Info.Sender}, whatsmeow.ParticipantChangeRemove)
	case "warn":
		s := getGroupSettings(v.Info.Chat.String())
		s.Warnings[v.Info.Sender.String()]++
		if s.Warnings[v.Info.Sender.String()] >= 3 {
			client.UpdateGroupParticipants(context.Background(), v.Info.Chat, []types.JID{v.Info.Sender}, whatsmeow.ParticipantChangeRemove)
			delete(s.Warnings, v.Info.Sender.String())
		}
		saveGroupSettings(s)
	}
}

func startSecuritySetup(client *whatsmeow.Client, v *events.Message, secType string) {
	if !v.Info.IsGroup || !isAdmin(client, v.Info.Chat, v.Info.Sender) { return }
	setupMap[v.Info.Sender.String()] = &SetupState{Type: secType, Stage: 1, GroupID: v.Info.Chat.String(), User: v.Info.Sender.String()}
	msg := fmt.Sprintf("╭━━━〔 %s SETUP 〕━━━┈\n┃ 🛡️ *Step 1: Allow Admins?*\n┃\n┃ 1️⃣ *Yes*\n┃ 2️⃣ *No*\n┃\n┃ _Reply with number_\n╰━━━━━━━━━━━━━━━━━━┈", strings.ToUpper(secType))
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{Conversation: proto.String(msg)})
}

func handleSetupResponse(client *whatsmeow.Client, v *events.Message, state *SetupState) {
	txt := strings.TrimSpace(getText(v.Message))
	s := getGroupSettings(state.GroupID)

	if state.Stage == 1 {
		if txt == "1" { s.AntilinkAdmin = true } else if txt == "2" { s.AntilinkAdmin = false } else { return }
		state.Stage = 2
		msg := "╭━━━〔 ACTION SETUP 〕━━━┈\n┃ ⚡ *Step 2: Choose Action*\n┃\n┃ 1️⃣ *Delete*\n┃ 2️⃣ *Kick*\n┃ 3️⃣ *Warn*\n┃\n┃ _Reply with number_\n╰━━━━━━━━━━━━━━━━━━┈"
		client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{Conversation: proto.String(msg)})
		return
	}

	if state.Stage == 2 {
		switch txt {
		case "1": s.AntilinkAction = "delete"
		case "2": s.AntilinkAction = "kick"
		case "3": s.AntilinkAction = "warn"
		default: return
		}
		switch state.Type {
		case "antilink": s.Antilink = true
		case "antipic": s.AntiPic = true
		case "antivideo": s.AntiVideo = true
		case "antisticker": s.AntiSticker = true
		}
		saveGroupSettings(s)
		delete(setupMap, state.User)
		cardReply(client, v.Info.Chat, "SUCCESS", strings.ToUpper(state.Type)+" is now active.")
	}
}

// ==================== گروپ سسٹم ====================
func handleKick(client *whatsmeow.Client, v *events.Message, args []string) {
	groupAction(client, v, args, "remove")
}

func handleAdd(client *whatsmeow.Client, v *events.Message, args []string) {
	if !v.Info.IsGroup || len(args) == 0 { return }
	jid, _ := types.ParseJID(args[0] + "@s.whatsapp.net")
	client.UpdateGroupParticipants(context.Background(), v.Info.Chat, []types.JID{jid}, whatsmeow.ParticipantChangeAdd)
	cardReply(client, v.Info.Chat, "GROUP", "User added.")
}

func handlePromote(client *whatsmeow.Client, v *events.Message, args []string) {
	groupAction(client, v, args, "promote")
}

func handleDemote(client *whatsmeow.Client, v *events.Message, args []string) {
	groupAction(client, v, args, "demote")
}

func handleTagAll(client *whatsmeow.Client, v *events.Message, args []string) {
	if !v.Info.IsGroup { return }
	info, _ := client.GetGroupInfo(context.Background(), v.Info.Chat)
	mentions := []string{}
	out := "📣 *TAG ALL*\n\n"
	if len(args) > 0 { out += strings.Join(args, " ") + "\n\n" }
	for _, p := range info.Participants {
		mentions = append(mentions, p.JID.String())
		out += "@" + p.JID.User + " "
	}
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(out),
			ContextInfo: &waProto.ContextInfo{MentionedJID: mentions},
		},
	})
}

func handleHideTag(client *whatsmeow.Client, v *events.Message, args []string) {
	if !v.Info.IsGroup { return }
	info, _ := client.GetGroupInfo(context.Background(), v.Info.Chat)
	mentions := []string{}
	for _, p := range info.Participants { mentions = append(mentions, p.JID.String()) }
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(strings.Join(args, " ")),
			ContextInfo: &waProto.ContextInfo{MentionedJID: mentions},
		},
	})
}

func handleGroup(client *whatsmeow.Client, v *events.Message, args []string) {
	if !v.Info.IsGroup || len(args) == 0 { return }
	switch args[0] {
	case "close": client.SetGroupAnnounce(context.Background(), v.Info.Chat, true)
	case "open": client.SetGroupAnnounce(context.Background(), v.Info.Chat, false)
	case "link":
		code, _ := client.GetGroupInviteLink(context.Background(), v.Info.Chat, false)
		cardReply(client, v.Info.Chat, "LINK", "https://chat.whatsapp.com/"+code)
	}
}

func handleDelete(client *whatsmeow.Client, v *events.Message) {
	if v.Message.ExtendedTextMessage != nil && v.Message.ExtendedTextMessage.ContextInfo != nil {
		ctx := v.Message.ExtendedTextMessage.ContextInfo
		client.RevokeMessage(context.Background(), v.Info.Chat, v.Info.Sender, *ctx.StanzaID)
	}
}

func groupAction(client *whatsmeow.Client, v *events.Message, args []string, action string) {
	if !v.Info.IsGroup { return }
	var target types.JID
	if len(args) > 0 {
		num := cleanNumber(args[0]) + "@s.whatsapp.net"
		target, _ = types.ParseJID(num)
	} else if v.Message.ExtendedTextMessage != nil && v.Message.ExtendedTextMessage.ContextInfo != nil {
		if v.Message.ExtendedTextMessage.ContextInfo.Participant != nil {
			target, _ = types.ParseJID(*v.Message.ExtendedTextMessage.ContextInfo.Participant)
		}
	}
	if target.IsEmpty() { return }
	
	var change whatsmeow.ParticipantChange
	switch action {
	case "remove": change = whatsmeow.ParticipantChangeRemove
	case "promote": change = whatsmeow.ParticipantChangePromote
	case "demote": change = whatsmeow.ParticipantChangeDemote
	}
	client.UpdateGroupParticipants(context.Background(), v.Info.Chat, []types.JID{target}, change)
	cardReply(client, v.Info.Chat, "GROUP", "Action successful.")
}

// ==================== ڈاؤن لوڈر سسٹم ====================
func handleTikTok(client *whatsmeow.Client, v *events.Message, url string) {
	react(client, v.Info.Chat, v.Info.ID, "🎵")
	var r struct { Data struct { Play string `json:"play"` } }
	getJson("https://www.tikwm.com/api/?url="+url, &r)
	if r.Data.Play != "" { sendVideo(client, v.Info.Chat, r.Data.Play, "🎵 TikTok") }
}

func handleFacebook(client *whatsmeow.Client, v *events.Message, url string) {
	react(client, v.Info.Chat, v.Info.ID, "📘")
	var r struct { BK9 struct { HD string } }
	getJson("https://bk9.fun/downloader/facebook?url="+url, &r)
	if r.BK9.HD != "" { sendVideo(client, v.Info.Chat, r.BK9.HD, "📘 Facebook") }
}

func handleInstagram(client *whatsmeow.Client, v *events.Message, url string) {
	react(client, v.Info.Chat, v.Info.ID, "📸")
	var r struct { Video struct { Url string } }
	getJson("https://api.tiklydown.eu.org/api/download?url="+url, &r)
	if r.Video.Url != "" { sendVideo(client, v.Info.Chat, r.Video.Url, "📸 Instagram") }
}

func handlePinterest(client *whatsmeow.Client, v *events.Message, url string) {
	react(client, v.Info.Chat, v.Info.ID, "📌")
	var r struct { BK9 struct { Url string } }
	getJson("https://bk9.fun/downloader/pinterest?url="+url, &r)
	if r.BK9.Url != "" { sendImage(client, v.Info.Chat, r.BK9.Url, "📌 Pinterest") }
}

func handleYouTubeMP3(client *whatsmeow.Client, v *events.Message, url string) {
	react(client, v.Info.Chat, v.Info.ID, "📺")
	var r struct { BK9 struct { Mp3 string } }
	getJson("https://bk9.fun/downloader/youtube?url="+url, &r)
	if r.BK9.Mp3 != "" { sendDocument(client, v.Info.Chat, r.BK9.Mp3, "audio.mp3", "audio/mpeg") }
}

func handleYouTubeMP4(client *whatsmeow.Client, v *events.Message, url string) {
	react(client, v.Info.Chat, v.Info.ID, "📺")
	var r struct { BK9 struct { Mp4 string } }
	getJson("https://bk9.fun/downloader/youtube?url="+url, &r)
	if r.BK9.Mp4 != "" { sendVideo(client, v.Info.Chat, r.BK9.Mp4, "📺 YouTube") }
}

// ==================== ٹولز سسٹم ====================
func handleSticker(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "🎨")
	d, _ := downloadMedia(client, v.Message)
	if d == nil { return }
	ioutil.WriteFile("t.jpg", d, 0644)
	exec.Command("ffmpeg", "-y", "-i", "t.jpg", "-vcodec", "libwebp", "t.webp").Run()
	b, _ := ioutil.ReadFile("t.webp")
	up, _ := client.Upload(context.Background(), b, whatsmeow.MediaImage)
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{StickerMessage: &waProto.StickerMessage{URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey, Mimetype: proto.String("image/webp")}})
}

func handleToImg(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "🖼️")
	d, _ := downloadMedia(client, v.Message)
	if d == nil { return }
	ioutil.WriteFile("t.webp", d, 0644)
	exec.Command("ffmpeg", "-y", "-i", "t.webp", "t.png").Run()
	b, _ := ioutil.ReadFile("t.png")
	up, _ := client.Upload(context.Background(), b, whatsmeow.MediaImage)
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{ImageMessage: &waProto.ImageMessage{URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey, Mimetype: proto.String("image/png")}})
}

func handleToVideo(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "🎥")
	d, _ := downloadMedia(client, v.Message)
	if d == nil { return }
	ioutil.WriteFile("t.webp", d, 0644)
	exec.Command("ffmpeg", "-y", "-i", "t.webp", "t.mp4").Run()
	b, _ := ioutil.ReadFile("t.mp4")
	up, _ := client.Upload(context.Background(), b, whatsmeow.MediaVideo)
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{VideoMessage: &waProto.VideoMessage{URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey, Mimetype: proto.String("video/mp4")}})
}

func handleRemoveBG(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "✂️")
	d, _ := downloadMedia(client, v.Message)
	u := uploadToCatbox(d)
	sendImage(client, v.Info.Chat, "https://bk9.fun/tools/removebg?url="+u, "✂️ Cleaned")
}

func handleRemini(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "✨")
	d, _ := downloadMedia(client, v.Message)
	u := uploadToCatbox(d)
	var r struct{ Url string }
	getJson("https://remini.mobilz.pw/enhance?url="+u, &r)
	sendImage(client, v.Info.Chat, r.Url, "✨ Enhanced")
}

func handleToURL(client *whatsmeow.Client, v *events.Message) {
	d, _ := downloadMedia(client, v.Message)
	cardReply(client, v.Info.Chat, "URL", uploadToCatbox(d))
}

func handleWeather(client *whatsmeow.Client, v *events.Message, city string) {
	react(client, v.Info.Chat, v.Info.ID, "🌦️")
	r, _ := http.Get("https://wttr.in/"+city+"?format=%C+%t")
	d, _ := ioutil.ReadAll(r.Body)
	cardReply(client, v.Info.Chat, "WEATHER", string(d))
}

func handleTranslate(client *whatsmeow.Client, v *events.Message, args []string) {
	t := strings.Join(args, " ")
	r, _ := http.Get("https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=ur&dt=t&q="+url.QueryEscape(t))
	var res []interface{}
	json.NewDecoder(r.Body).Decode(&res)
	if len(res)>0 { cardReply(client, v.Info.Chat, "TRANSLATE", res[0].([]interface{})[0].([]interface{})[0].(string)) }
}

func handleVV(client *whatsmeow.Client, v *events.Message) {
	react(client, v.Info.Chat, v.Info.ID, "🫣")
	q := v.Message.ExtendedTextMessage.GetContextInfo().GetQuotedMessage()
	if q == nil { return }
	d, _ := downloadMedia(client, &waProto.Message{ImageMessage: q.ImageMessage, VideoMessage: q.VideoMessage, ViewOnceMessage: q.ViewOnceMessage})
	if q.ImageMessage != nil || (q.ViewOnceMessage != nil && q.ViewOnceMessage.Message.ImageMessage != nil) {
		sendImage(client, v.Info.Chat, uploadToCatbox(d), "🫣 VV Unlocked")
	} else {
		sendVideo(client, v.Info.Chat, uploadToCatbox(d), "🫣 VV Unlocked")
	}
}

// ==================== ہیلپر فنکشنز (FIXED) ====================
func react(client *whatsmeow.Client, chat types.JID, msgID types.MessageID, emoji string) {
	client.SendMessage(context.Background(), chat, &waProto.Message{
		ReactionMessage: &waProto.ReactionMessage{
			Key: &waProto.MessageKey{RemoteJID: proto.String(chat.String()), ID: proto.String(string(msgID)), FromMe: proto.Bool(false)},
			Text: proto.String(emoji),
		},
	})
}

func getText(m *waProto.Message) string {
	if m == nil { return "" }
	if m.Conversation != nil { return *m.Conversation }
	if m.ExtendedTextMessage != nil { return *m.ExtendedTextMessage.Text }
	if m.ImageMessage != nil { return *m.ImageMessage.Caption }
	if m.VideoMessage != nil { return *m.VideoMessage.Caption }
	return ""
}

func isOwner(client *whatsmeow.Client, sender types.JID) bool {
	return cleanNumber(client.Store.ID.User) == cleanNumber(sender.User)
}

func cleanNumber(num string) string {
	num = strings.ReplaceAll(num, "+", "")
	if strings.Contains(num, ":") { num = strings.Split(num, ":")[0] }
	if strings.Contains(num, "@") { num = strings.Split(num, "@")[0] }
	return num
}

func canExecute(client *whatsmeow.Client, v *events.Message, cmd string) bool {
	if isOwner(client, v.Info.Sender) { return true }
	if !v.Info.IsGroup { return true }
	s := getGroupSettings(v.Info.Chat.String())
	if s.Mode == "private" { return false }
	if s.Mode == "admin" { return isAdmin(client, v.Info.Chat, v.Info.Sender) }
	return true
}

func isAdmin(client *whatsmeow.Client, chat, user types.JID) bool {
	info, err := client.GetGroupInfo(context.Background(), chat)
	if err != nil { return false }
	for _, p := range info.Participants {
		if p.JID.User == user.User && (p.IsAdmin || p.IsSuperAdmin) { return true }
	}
	return false
}

func getGroupSettings(id string) *GroupSettings {
	cacheMutex.RLock()
	if s, ok := groupCache[id]; ok { cacheMutex.RUnlock(); return s }
	cacheMutex.RUnlock()
	s := &GroupSettings{ChatID: id, Mode: "public", AntilinkAdmin: true, AntilinkAction: "delete", Warnings: make(map[string]int)}
	cacheMutex.Lock()
	groupCache[id] = s
	cacheMutex.Unlock()
	return s
}

func saveGroupSettings(s *GroupSettings) {
	cacheMutex.Lock()
	groupCache[s.ChatID] = s
	cacheMutex.Unlock()
}

func containsLink(t string) bool {
	l := strings.ToLower(t)
	return strings.Contains(l, "http://") || strings.Contains(l, "https://") || strings.Contains(l, "chat.whatsapp.com")
}

// ==================== مونو ڈیٹا فنکشنز (FIXED) ====================
func loadDataFromMongo() {
	if mongoColl == nil { return }
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var result BotData
	err := mongoColl.FindOne(ctx, bson.M{"_id": "bot_config"}).Decode(&result)
	if err != nil {
		data = BotData{ID: "bot_config", Prefix: "."}
		saveBotData()
	} else {
		data = result
	}
}

func saveBotData() {
	if mongoColl == nil { return }
	dataMutex.RLock()
	defer dataMutex.RUnlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	opts := options.Replace().SetUpsert(true)
	mongoColl.ReplaceOne(ctx, bson.M{"_id": data.ID}, data, opts)
}

// ==================== میڈیا ہیلپرز ====================
func getJson(u string, t interface{}) error {
	r, err := http.Get(u); if err != nil { return err }
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(t)
}

func downloadMedia(client *whatsmeow.Client, m *waProto.Message) ([]byte, error) {
	var d whatsmeow.DownloadableMessage
	if m.ImageMessage != nil { d = m.ImageMessage } else if m.VideoMessage != nil { d = m.VideoMessage } else if m.StickerMessage != nil { d = m.StickerMessage }
	if d == nil { return nil, fmt.Errorf("no media") }
	return client.Download(context.Background(), d)
}

func uploadToCatbox(d []byte) string {
	b := new(bytes.Buffer); w := multipart.NewWriter(b)
	p, _ := w.CreateFormFile("fileToUpload", "f.jpg"); p.Write(d)
	w.WriteField("reqtype", "fileupload"); w.Close()
	r, _ := http.Post("https://catbox.moe/user/api.php", w.FormDataContentType(), b)
	res, _ := ioutil.ReadAll(r.Body); return string(res)
}

func sendVideo(client *whatsmeow.Client, chat types.JID, u, c string) {
	r, _ := http.Get(u); d, _ := ioutil.ReadAll(r.Body)
	up, _ := client.Upload(context.Background(), d, whatsmeow.MediaVideo)
	client.SendMessage(context.Background(), chat, &waProto.Message{VideoMessage: &waProto.VideoMessage{URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey, Mimetype: proto.String("video/mp4"), Caption: proto.String(c)}})
}

func sendImage(client *whatsmeow.Client, chat types.JID, u, c string) {
	r, _ := http.Get(u); d, _ := ioutil.ReadAll(r.Body)
	up, _ := client.Upload(context.Background(), d, whatsmeow.MediaImage)
	client.SendMessage(context.Background(), chat, &waProto.Message{ImageMessage: &waProto.ImageMessage{URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey, Mimetype: proto.String("image/jpeg"), Caption: proto.String(c)}})
}

func sendDocument(client *whatsmeow.Client, chat types.JID, u, n, m string) {
	r, _ := http.Get(u); d, _ := ioutil.ReadAll(r.Body)
	up, _ := client.Upload(context.Background(), d, whatsmeow.MediaDocument)
	client.SendMessage(context.Background(), chat, &waProto.Message{DocumentMessage: &waProto.DocumentMessage{URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey, Mimetype: proto.String(m), FileName: proto.String(n)}})
}
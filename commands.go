package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// ═══════════════════════════════════════════════════════════════
// 🌐 GLOBAL VARIABLES
// ═══════════════════════════════════════════════════════════════

var (
	activeClients = make(map[string]*whatsmeow.Client)
	clientsMutex  sync.RWMutex
	startTime     = time.Now()
)

// ═══════════════════════════════════════════════════════════════
// 📡 MAIN EVENT HANDLER
// ═══════════════════════════════════════════════════════════════

func handler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		// Client کو event سے extract کریں
		go processMessage(nil, v) // Client ko properly pass karna hoga
	case *events.GroupInfo:
		go handleGroupInfoChange(nil, v)
	}
}

// یہ فنکشن چیک کرتا ہے کہ آیا میسج میں موجود لفظ ہماری لسٹ میں ہے یا نہیں
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
			client.MarkRead([]types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender)
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
		client.MarkRead([]types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender)
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

	if !strings.HasPrefix(body, prefix) && !isKnownCommand(body) {
		return
	}

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

	// 🔐 PERMISSION CHECK (UPDATED LID LOGIC)
	if !canExecute(client, v, cmd) {
		return
	}

	fullArgs := strings.Join(args, " ")
	fmt.Printf("📩 CMD: %s | User: %s | Chat: %s\n", cmd, v.Info.Sender.User, v.Info.Chat.User)

	switch cmd {
	case "menu", "help", "list":
		react(client, v.Info.Chat, v.Info.ID, "📜")
		sendMenu(client, v)
	case "ping":
		react(client, v.Info.Chat, v.Info.ID, "⚡")
		sendPing(client, v)
	case "id":
		react(client, v.Info.Chat, v.Info.ID, "🆔")
		sendID(client, v)
	case "owner":
		react(client, v.Info.Chat, v.Info.ID, "👑")
		sendOwner(client, v)
	case "listbots":
		react(client, v.Info.Chat, v.Info.ID, "📊")
		sendBotsList(client, v)
	case "data":
		replyMessage(client, v, "╔════════════════╗\n║ 📂 DATA STATUS\n╠════════════════╣\n║ ✅ DB Coming\n╚════════════════╝")
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
		handleAddStatus(client, v, args)
	case "delstatus":
		handleDelStatus(client, v, args)
	case "liststatus":
		handleListStatus(client, v)
	case "readallstatus":
		handleReadAllStatus(client, v)
	case "setprefix":
		handleSetPrefix(client, v, args)
	case "mode":
		handleMode(client, v, args)
	case "antilink":
		startSecuritySetup(client, v, "antilink")
	case "antipic":
		startSecuritySetup(client, v, "antipic")
	case "antivideo":
		startSecuritySetup(client, v, "antivideo")
	case "antisticker":
		startSecuritySetup(client, v, "antisticker")
	case "kick":
		handleKick(client, v, args)
	case "add":
		handleAdd(client, v, args)
	case "promote":
		handlePromote(client, v, args)
	case "demote":
		handleDemote(client, v, args)
	case "tagall":
		handleTagAll(client, v, args)
	case "hidetag":
		handleHideTag(client, v, args)
	case "group":
		handleGroup(client, v, args)
	case "del", "delete":
		handleDelete(client, v)
	case "sticker", "s":
		handleSticker(client, v)
	case "toimg":
		handleToImg(client, v)
	case "tovideo":
		handleToVideo(client, v)
	case "removebg":
		handleRemoveBG(client, v)
	case "remini":
		handleRemini(client, v)
	case "tourl":
		handleToURL(client, v)
	case "weather":
		handleWeather(client, v, fullArgs)
	case "translate", "tr":
		handleTranslate(client, v, args)
	case "vv":
		handleVV(client, v)
	}
}

// ═══════════════════════════════════════════════════════════════
// 🔐 SECURITY & OWNER LOGIC (LID BASED)
// ═══════════════════════════════════════════════════════════════

// کلین آئی ڈی نکالنے کا فنکشن - صرف نمبر یا LID
func getCleanID(jidStr string) string {
	if jidStr == "" {
		return "unknown"
	}
	
	// @ کے پہلے والا حصہ نکالیں
	parts := strings.Split(jidStr, "@")
	if len(parts) == 0 {
		return "unknown"
	}
	
	userPart := parts[0]
	
	// ڈیوائس آئی ڈی ہٹائیں (جیسے :61 یا .0:1)
	if strings.Contains(userPart, ":") {
		colonParts := strings.Split(userPart, ":")
		userPart = colonParts[0]
	}
	
	// ڈاٹ والا حصہ بھی ہٹائیں
	if strings.Contains(userPart, ".") {
		dotParts := strings.Split(userPart, ".")
		userPart = dotParts[0]
	}
	
	return strings.TrimSpace(userPart)
}

// بوٹ کی LID نکالنے کا فنکشن
func getBotLID(client *whatsmeow.Client) string {
	if client.Store.ID == nil {
		return "unknown"
	}
	
	// پہلے LID چیک کریں (اگر موجود ہے)
	// LID ایک JID type ہے، اس لیے String() method استعمال کریں
	lidStr := client.Store.LID.String()
	if lidStr != "" {
		cleanLID := getCleanID(lidStr)
		fmt.Printf("🔍 [BOT LID] Raw: %s | Clean: %s\n", lidStr, cleanLID)
		return cleanLID
	}
	
	// اگر LID نہیں ملی تو نارمل ID استعمال کریں
	cleanID := getCleanID(client.Store.ID.User)
	fmt.Printf("🔍 [BOT ID] Raw: %s | Clean: %s\n", client.Store.ID.User, cleanID)
	return cleanID
}

// اونر چیک کرنے کا بہتر فنکشن
func isOwner(client *whatsmeow.Client, sender types.JID) bool {
	if client.Store.ID == nil {
		fmt.Println("⚠️ [OWNER CHECK] Client Store ID is nil")
		return false
	}
	
	// سینڈر کا کلین نمبر/آئی ڈی
	senderClean := getCleanID(sender.String())
	
	// بوٹ کا اپنا کلین نمبر (Store.ID.User سے)
	botNumClean := getCleanID(client.Store.ID.User)
	
	// بوٹ کی کلین LID (اگر موجود ہے)
	botLIDClean := ""
	lidStr := client.Store.LID.String()
	if lidStr != "" {
		botLIDClean = getCleanID(lidStr)
	}
	
	// میچنگ لوجک: سینڈر بوٹ کا نمبر ہے یا بوٹ کی LID ہے
	isMatch := false
	matchType := "NONE"
	
	if senderClean == botNumClean {
		isMatch = true
		matchType = "NUMBER"
	} else if botLIDClean != "" && senderClean == botLIDClean {
		isMatch = true
		matchType = "LID"
	}
	
	// تفصیلی لاگ
	fmt.Printf(`
╔═══════════════════════════════════╗
║ 🎯 OWNER VERIFICATION CHECK
╠═══════════════════════════════════╣
║ 👤 Sender Clean : %s
║ 🤖 Bot Number   : %s
║ 🆔 Bot LID      : %s
║ 📊 Match Type   : %s
║ ✅ Is Owner     : %v
╚═══════════════════════════════════╝
`, senderClean, botNumClean, botLIDClean, matchType, isMatch)
	
	return isMatch
}

// ایڈمن چیک کرنے کا فنکشن
func isAdmin(client *whatsmeow.Client, chat, user types.JID) bool {
	info, err := client.GetGroupInfo(chat)
	if err != nil {
		return false
	}
	
	userClean := getCleanID(user.String())
	
	for _, p := range info.Participants {
		participantClean := getCleanID(p.JID.String())
		if participantClean == userClean && (p.IsAdmin || p.IsSuperAdmin) {
			return true
		}
	}
	return false
}

// کمانڈ ایگزیکیوٹ کرنے کی اجازت چیک کریں
func canExecute(client *whatsmeow.Client, v *events.Message, cmd string) bool {
	// اگر اونر ہے تو ہر کمانڈ چلا سکتا ہے
	if isOwner(client, v.Info.Sender) {
		return true
	}
	
	// اگر پرائیویٹ چیٹ ہے تو سب کو اجازت ہے
	if !v.Info.IsGroup {
		return true
	}
	
	// گروپ سیٹنگز چیک کریں
	s := getGroupSettings(v.Info.Chat.String())
	
	if s.Mode == "private" {
		return false
	}
	
	if s.Mode == "admin" {
		return isAdmin(client, v.Info.Chat, v.Info.Sender)
	}
	
	return true
}

// ═══════════════════════════════════════════════════════════════
// 📜 HELPERS & UI
// ═══════════════════════════════════════════════════════════════

func sendOwner(client *whatsmeow.Client, v *events.Message) {
	isOwn := isOwner(client, v.Info.Sender)
	status := "❌ NOT Owner"
	emoji := "🚫"
	
	if isOwn {
		status = "✅ YOU are Owner"
		emoji = "👑"
	}
	
	// بوٹ کی تفصیلات
	botNum := getCleanID(client.Store.ID.User)
	botLID := "N/A"
	lidStr := client.Store.LID.String()
	if lidStr != "" {
		botLID = getCleanID(lidStr)
	}
	
	senderClean := getCleanID(v.Info.Sender.String())
	
	msg := fmt.Sprintf(`╔═══════════════════════════╗
║ %s OWNER VERIFICATION
╠═══════════════════════════╣
║ 🤖 Bot Number  : %s
║ 🆔 Bot LID     : %s
║ 👤 Your ID     : %s
╠═══════════════════════════╣
║ 📊 Status: %s
╠═══════════════════════════╣
║ 💡 Tip: LID-based security
║    ensures multi-device
║    owner recognition!
╚═══════════════════════════╝`, emoji, botNum, botLID, senderClean, status)
	
	replyMessage(client, v, msg)
}

func sendBotsList(client *whatsmeow.Client, v *events.Message) {
	clientsMutex.RLock()
	count := len(activeClients)
	
	msg := fmt.Sprintf(`╔═══════════════════════════╗
║ 📊 MULTI-BOT STATUS
╠═══════════════════════════╣
║ 🤖 Active Bots: %d
║ 🔄 Auto-Connect: ✅
║ 🔐 LID Security: ✅
║ 📡 DB Sync: ✅
╠═══════════════════════════╣`, count)
	
	i := 1
	for num := range activeClients {
		msg += fmt.Sprintf("\n║ %d. %s", i, num)
		i++
	}
	
	clientsMutex.RUnlock()
	
	msg += "\n╚═══════════════════════════╝"
	
	replyMessage(client, v, msg)
}

// ═══════════════════════════════════════════════════════════════
// 📜 MENU SYSTEM
// ═══════════════════════════════════════════════════════════════

func sendMenu(client *whatsmeow.Client, v *events.Message) {
	uptime := time.Since(startTime).Round(time.Second)
	dataMutex.RLock()
	p := data.Prefix
	dataMutex.RUnlock()

	s := getGroupSettings(v.Info.Chat.String())
	currentMode := strings.ToUpper(s.Mode)
	if !strings.Contains(v.Info.Chat.String(), "@g.us") {
		currentMode = "PRIVATE"
	}

	menu := fmt.Sprintf(`╔═════════════════╗
║   %s   
╠═════════════════╣
║ 👋 *Assalam-o-Alaikum*     
║ 👑 *Owner:* %s             
║ 🛡️ *Mode:* %s              
║ ⏳ *Uptime:* %s            
╠═════════════════╣
║                          
║  ╭─────── DOWNLOADERS─╮
║  │ 🔸 *%sfb* - Facebook   
║  │ 🔸 *%sig* - Instagram  
║  │ 🔸 *%spin* - Pinterest 
║  │ 🔸 *%stiktok* - TikTok 
║  │ 🔸 *%sytmp3* - YT Audio
║  │ 🔸 *%sytmp4* - YT Video 
║  ╰───────────────────╯
║                           
║  ╭─────── GROUP ──────╮
║  │ 🔸 *%sadd* - Add Member
║  │ 🔸 *%sdemote* - Demote 
║  │ 🔸 *%sgroup* - Settings
║  │ 🔸 *%shidetag* - Hidden
║  │ 🔸 *%skick* - Remove   
║  │ 🔸 *%spromote* - Admin
║  │ 🔸 *%stagall* - Mention
║  ╰───────────────────╯
║                           
║  ╭──── SETTINGS ───╮
║  │ 🔸 *%saddstatus*       
║  │ 🔸 *%salwaysonline*     
║  │ 🔸 *%santilink*         
║  │ 🔸 *%santipic*         
║  │ 🔸 *%santisticker*     
║  │ 🔸 *%santivideo*        
║  │ 🔸 *%sautoreact*    
║  │ 🔸 *%sautoread*      
║  │ 🔸 *%sautostatus*   
║  │ 🔸 *%sdelstatus*    
║  │ 🔸 *%sliststatus*   
║  │ 🔸 *%smode*      
║  │ 🔸 *%sowner*     
║  │ 🔸 *%sreadallstatus* 
║  │ 🔸 *%sstatusreact*  
║  ╰─────────────────╯
║                           
║  ╭─────── TOOLS ───────╮
║  │ 🔸 *%sdata* - DB Status
║  │ 🔸 *%sid* - Get ID      
║  │ 🔸 *%slistbots* - Bots🆕
║  │ 🔸 *%sping* - Speed     
║  │ 🔸 *%sremini* - Enhance
║  │ 🔸 *%sremovebg* - BG  
║  │ 🔸 *%ssticker* - Create 
║  │ 🔸 *%stoimg* - Convert 
║  │ 🔸 *%stourl* - Upload  
║  │ 🔸 *%stovideo* - Make 
║  │ 🔸 *%stranslate* - Lang
║  │ 🔸 *%svv* - ViewOnce 
║  │ 🔸 *%sweather* - Info
║  ╰────────────────────╯
║                          
╠═════════════════════╣
║ 🔐 LID-Based Security
║ © 2025 Nothing is Impossible 
╚═════════════════════╝`,
		BOT_NAME, OWNER_NAME, currentMode, uptime,
		p, p, p, p, p, p,
		p, p, p, p, p, p, p,
		p, p, p, p, p, p, p, p, p, p, p, p, p, p, p,
		p, p, p, p, p, p, p, p, p, p, p, p, p)

	sendReplyMessage(client, v, menu)
}

// ═══════════════════════════════════════════════════════════════
// 📜 REMAINING UI FUNCTIONS
// ═══════════════════════════════════════════════════════════════

func sendPing(client *whatsmeow.Client, v *events.Message) {
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	ms := time.Since(start).Milliseconds()
	uptime := time.Since(startTime).Round(time.Second)

	msg := fmt.Sprintf(`╔════════════════╗
║ ⚡ PING STATUS
╠════════════════╣
║ 🚀 Speed: %d MS
║ ⏱️ Uptime: %s
║ 👑 Dev: %s
╠════════════════╣
║ 🟢 System Running
╚════════════════╝`, ms, uptime, OWNER_NAME)

	sendReplyMessage(client, v, msg)
}

func sendID(client *whatsmeow.Client, v *events.Message) {
	user := v.Info.Sender.User
	chat := v.Info.Chat.User
	chatType := "Private"
	if v.Info.IsGroup {
		chatType = "Group"
	}

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

// ═══════════════════════════════════════════════════════════════
// 🛠️ HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════

func react(client *whatsmeow.Client, chat types.JID, msgID types.MessageID, emoji string) {
	client.SendMessage(context.Background(), chat, &waProto.Message{
		ReactionMessage: &waProto.ReactionMessage{
			Key: &waProto.MessageKey{
				RemoteJID: proto.String(chat.String()),
				ID:        proto.String(string(msgID)),
				FromMe:    proto.Bool(false),
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
	if m.Conversation != nil {
		return *m.Conversation
	}
	if m.ExtendedTextMessage != nil && m.ExtendedTextMessage.Text != nil {
		return *m.ExtendedTextMessage.Text
	}
	if m.ImageMessage != nil && m.ImageMessage.Caption != nil {
		return *m.ImageMessage.Caption
	}
	if m.VideoMessage != nil && m.VideoMessage.Caption != nil {
		return *m.VideoMessage.Caption
	}
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
		AntiPic:        false,
		AntiVideo:      false,
		AntiSticker:    false,
		Warnings:       make(map[string]int),
	}

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

// ═══════════════════════════════════════════════════════════════
// 🚀 MULTI-BOT BOOTSTRAP (POSTGRES + AUTO-CONNECT)
// ═══════════════════════════════════════════════════════════════

// نیا سیشن کنیکٹ کرنے کا فنکشن
func ConnectNewSession(device *store.Device) {
	clientLog := waLog.Stdout("Client", "DEBUG", true)
	client := whatsmeow.NewClient(device, clientLog)
	
	// Event handler add کریں
	client.AddEventHandler(func(evt interface{}) {
		handler(evt)
	})

	botID := getCleanID(device.ID.User)
	
	err := client.Connect()
	if err != nil {
		fmt.Printf("❌ [MULTI-BOT] نمبر %s کنیکٹ نہیں ہو سکا: %v\n", botID, err)
		return
	}

	// کلائنٹ کو سیو کریں
	clientsMutex.Lock()
	activeClients[botID] = client
	clientsMutex.Unlock()

	lidStr := device.LID.String()
	fmt.Printf(`
╔═══════════════════════════════════╗
║ ✅ BOT CONNECTED SUCCESSFULLY!
╠═══════════════════════════════════╣
║ 📱 Number: %s
║ 🆔 LID: %s
║ 🕐 Time: %s
╚═══════════════════════════════════╝
`, botID, getCleanID(lidStr), time.Now().Format("15:04:05"))
}

// تمام بوٹس کو اسٹارٹ کرنے کا فنکشن
func StartAllBots(container *sqlstore.Container) {
	ctx := context.Background()
	
	devices, err := container.GetAllDevices(ctx)
	if err != nil {
		fmt.Printf("❌ [MULTI-BOT] ڈیٹا بیس سے سیشن لوڈ کرنے میں غلطی: %v\n", err)
		return
	}

	if len(devices) == 0 {
		fmt.Println("⚠️ [MULTI-BOT] کوئی سیشن نہیں ملا! نیا سیشن بنائیں۔")
		return
	}

	fmt.Printf(`
╔═══════════════════════════════════╗
║ 🚀 MULTI-BOT SYSTEM STARTING
╠═══════════════════════════════════╣
║ 📂 Found: %d session(s) in DB
║ 🔄 Connecting all bots...
╚═══════════════════════════════════╝
`, len(devices))

	// ہر ڈیوائس کو الگ goroutine میں کنیکٹ کریں
	var wg sync.WaitGroup
	for i, device := range devices {
		wg.Add(1)
		go func(idx int, dev *store.Device) {
			defer wg.Done()
			
			fmt.Printf("\n[%d/%d] 🔌 کنیکٹ ہو رہا ہے: %s...\n", idx+1, len(devices), getCleanID(dev.ID.User))
			ConnectNewSession(dev)
			
			// تھوڑا سا وقفہ دیں تاکہ WhatsApp سرور پر زیادہ لوڈ نہ ہو
			time.Sleep(2 * time.Second)
		}(i, device)
	}

	// تمام connections مکمل ہونے کا انتظار کریں
	wg.Wait()

	clientsMutex.RLock()
	activeCount := len(activeClients)
	clientsMutex.RUnlock()

	fmt.Printf(`
╔═══════════════════════════════════╗
║ ✅ MULTI-BOT SYSTEM READY!
╠═══════════════════════════════════╣
║ 🤖 Active Bots: %d/%d
║ 🔐 LID Security: Enabled
║ 📡 Auto-Connect: Active
║ 💾 Database: PostgreSQL
╠═══════════════════════════════════╣
║ 💡 نئے سیشن خودکار طور پر
║    کنیکٹ ہو جائیں گے!
╚═══════════════════════════════════╝
`, activeCount, len(devices))

	// نئے سیشنز کی auto-monitoring شروع کریں
	go monitorNewSessions(container)
}

// نئے سیشنز کی نگرانی (Auto-Connect)
func monitorNewSessions(container *sqlstore.Container) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	fmt.Println("\n🔍 [AUTO-CONNECT] نئے سیشنز کی نگرانی شروع...")

	for range ticker.C {
		ctx := context.Background()
		devices, err := container.GetAllDevices(ctx)
		if err != nil {
			continue
		}

		for _, device := range devices {
			botID := getCleanID(device.ID.User)
			
			clientsMutex.RLock()
			_, exists := activeClients[botID]
			clientsMutex.RUnlock()

			// اگر یہ سیشن پہلے سے کنیکٹ نہیں ہے تو کنیکٹ کریں
			if !exists {
				fmt.Printf("\n🆕 [AUTO-CONNECT] نیا سیشن ملا: %s\n", botID)
				go ConnectNewSession(device)
				time.Sleep(3 * time.Second)
			}
		}
	}
}

// ═══════════════════════════════════════════════════════════════
// 🔧 ADDITIONAL HELPER TO GET CLIENT FROM ACTIVE CLIENTS
// ═══════════════════════════════════════════════════════════════

// کسی خاص JID کے لیے client نکالیں
func getClientForJID(jid types.JID) *whatsmeow.Client {
	cleanID := getCleanID(jid.String())
	
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()
	
	if client, ok := activeClients[cleanID]; ok {
		return client
	}
	
	return nil
}
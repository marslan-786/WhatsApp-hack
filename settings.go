package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// ==================== سیٹنگز سسٹم ====================
func toggleAlwaysOnline(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔═══════════════════════════╗
║      ❌ ACCESS DENIED      ║
╠═══════════════════════════╣
║  🔒 Owner Only Command    ║
╚═══════════════════════════╝`
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

	msg := fmt.Sprintf(`╔═══════════════════════════╗
║   ⚙️ ALWAYS ONLINE UPDATED  ║
╠═══════════════════════════╣
║                           ║
║  📊 *Status:* %s          ║
║  🔄 *State:* %s           ║
║  ✅ *Successfully Updated* ║
║                           ║
╚═══════════════════════════╝`, status, statusText)

	replyMessage(client, v, msg)
}

func toggleAutoRead(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔═══════════════════════════╗
║      ❌ ACCESS DENIED      ║
╠═══════════════════════════╣
║  🔒 Owner Only Command    ║
╚═══════════════════════════╝`
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

	msg := fmt.Sprintf(`╔═══════════════════════════╗
║    ⚙️ AUTO READ UPDATED     ║
╠═══════════════════════════╣
║                           ║
║  📊 *Status:* %s          ║
║  🔄 *State:* %s           ║
║  ✅ *Successfully Updated* ║
║                           ║
╚═══════════════════════════╝`, status, statusText)

	replyMessage(client, v, msg)
}

func toggleAutoReact(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔═══════════════════════════╗
║      ❌ ACCESS DENIED      ║
╠═══════════════════════════╣
║  🔒 Owner Only Command    ║
╚═══════════════════════════╝`
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

	msg := fmt.Sprintf(`╔═══════════════════════════╗
║   ⚙️ AUTO REACT UPDATED     ║
╠═══════════════════════════╣
║                           ║
║  📊 *Status:* %s          ║
║  🔄 *State:* %s           ║
║  ✅ *Successfully Updated* ║
║                           ║
╚═══════════════════════════╝`, status, statusText)

	replyMessage(client, v, msg)
}

func toggleAutoStatus(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔═══════════════════════════╗
║      ❌ ACCESS DENIED      ║
╠═══════════════════════════╣
║  🔒 Owner Only Command    ║
╚═══════════════════════════╝`
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

	msg := fmt.Sprintf(`╔═══════════════════════════╗
║   ⚙️ AUTO STATUS UPDATED    ║
╠═══════════════════════════╣
║                           ║
║  📊 *Status:* %s          ║
║  🔄 *State:* %s           ║
║  ✅ *Successfully Updated* ║
║                           ║
╚═══════════════════════════╝`, status, statusText)

	replyMessage(client, v, msg)
}

func toggleStatusReact(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔═══════════════════════════╗
║      ❌ ACCESS DENIED      ║
╠═══════════════════════════╣
║  🔒 Owner Only Command    ║
╚═══════════════════════════╝`
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

	msg := fmt.Sprintf(`╔═══════════════════════════╗
║  ⚙️ STATUS REACT UPDATED    ║
╠═══════════════════════════╣
║                           ║
║  📊 *Status:* %s          ║
║  🔄 *State:* %s           ║
║  ✅ *Successfully Updated* ║
║                           ║
╚═══════════════════════════╝`, status, statusText)

	replyMessage(client, v, msg)
}

func handleAddStatus(client *whatsmeow.Client, v *events.Message, args []string) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔═══════════════════════════╗
║      ❌ ACCESS DENIED      ║
╠═══════════════════════════╣
║  🔒 Owner Only Command    ║
╚═══════════════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	if len(args) < 1 {
		msg := `╔═══════════════════════════╗
║    ⚠️ INVALID FORMAT       ║
╠═══════════════════════════╣
║  📝 Usage:                ║
║     .addstatus <number>   ║
║                           ║
║  💡 Example:              ║
║     .addstatus 923001234567║
╚═══════════════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	num := args[0]
	dataMutex.Lock()
	data.StatusTargets = append(data.StatusTargets, num)
	dataMutex.Unlock()

	msg := fmt.Sprintf(`╔═══════════════════════════╗
║   ✅ STATUS TARGET ADDED    ║
╠═══════════════════════════╣
║                           ║
║  📱 *Number:* %s          ║
║  📊 *Total Targets:* %d   ║
║                           ║
╚═══════════════════════════╝`, num, len(data.StatusTargets))

	replyMessage(client, v, msg)
}

func handleDelStatus(client *whatsmeow.Client, v *events.Message, args []string) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔═══════════════════════════╗
║      ❌ ACCESS DENIED      ║
╠═══════════════════════════╣
║  🔒 Owner Only Command    ║
╚═══════════════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	if len(args) < 1 {
		msg := `╔═══════════════════════════╗
║    ⚠️ INVALID FORMAT       ║
╠═══════════════════════════╣
║  📝 Usage:                ║
║     .delstatus <number>   ║
║                           ║
║  💡 Example:              ║
║     .delstatus 923001234567║
╚═══════════════════════════╝`
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
		msg := fmt.Sprintf(`╔═══════════════════════════╗
║  ✅ STATUS TARGET REMOVED   ║
╠═══════════════════════════╣
║                           ║
║  📱 *Number:* %s          ║
║  📊 *Remaining:* %d       ║
║                           ║
╚═══════════════════════════╝`, num, len(data.StatusTargets))
		replyMessage(client, v, msg)
	} else {
		msg := `╔═══════════════════════════╗
║    ❌ NUMBER NOT FOUND     ║
╠═══════════════════════════╣
║  Target number not in list║
╚═══════════════════════════╝`
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
		msg := `╔═══════════════════════════╗
║   📭 NO STATUS TARGETS     ║
╠═══════════════════════════╣
║  No numbers configured yet║
║  Use .addstatus to add    ║
╚═══════════════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	msg := "╔═══════════════════════════╗\n"
	msg += "║   📜 STATUS TARGETS LIST   ║\n"
	msg += "╠═══════════════════════════╣\n"
	msg += "║                           ║\n"
	for i, t := range targets {
		msg += fmt.Sprintf("║  %d. %s\n", i+1, t)
	}
	msg += "║                           ║\n"
	msg += fmt.Sprintf("║  📊 *Total:* %d targets   ║\n", len(targets))
	msg += "╚═══════════════════════════╝"

	replyMessage(client, v, msg)
}

func handleSetPrefix(client *whatsmeow.Client, v *events.Message, args []string) {
	if !isOwner(client, v.Info.Sender) {
		msg := `╔═══════════════════════════╗
║      ❌ ACCESS DENIED      ║
╠═══════════════════════════╣
║  🔒 Owner Only Command    ║
╚═══════════════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	if len(args) < 1 {
		msg := `╔═══════════════════════════╗
║    ⚠️ INVALID FORMAT       ║
╠═══════════════════════════╣
║  📝 Usage:                ║
║     .setprefix <symbol>   ║
║                           ║
║  💡 Examples:             ║
║     .setprefix .          ║
║     .setprefix !          ║
║     .setprefix #          ║
╚═══════════════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	newPrefix := args[0]
	dataMutex.Lock()
	data.Prefix = newPrefix
	dataMutex.Unlock()

	msg := fmt.Sprintf(`╔═══════════════════════════╗
║   ✅ PREFIX UPDATED         ║
╠═══════════════════════════╣
║                           ║
║  🔧 *New Prefix:* %s      ║
║  ✅ *Successfully Changed* ║
║                           ║
║  💡 *Example:*            ║
║     %smenu               ║
║                           ║
╚═══════════════════════════╝`, newPrefix, newPrefix)

	replyMessage(client, v, msg)
}

func handleMode(client *whatsmeow.Client, v *events.Message, args []string) {
	if !v.Info.IsGroup {
		msg := `╔═══════════════════════════╗
║    ❌ GROUP ONLY COMMAND   ║
╠═══════════════════════════╣
║  This command works only  ║
║  in group chats           ║
╚═══════════════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	if !isAdmin(client, v.Info.Chat, v.Info.Sender) && !isOwner(client, v.Info.Sender) {
		msg := `╔═══════════════════════════╗
║      ❌ ACCESS DENIED      ║
╠═══════════════════════════╣
║  🔒 Admin Only Command    ║
╚═══════════════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	if len(args) < 1 {
		msg := `╔═══════════════════════════╗
║    ⚙️ GROUP MODE SETTINGS   ║
╠═══════════════════════════╣
║                           ║
║  📝 Available Modes:      ║
║                           ║
║  1️⃣ *public*              ║
║     Everyone can use      ║
║                           ║
║  2️⃣ *private*             ║
║     Bot disabled          ║
║                           ║
║  3️⃣ *admin*               ║
║     Admin only access     ║
║                           ║
║  💡 Usage:                ║
║     .mode <type>          ║
║                           ║
╚═══════════════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	mode := strings.ToLower(args[0])
	if mode != "public" && mode != "private" && mode != "admin" {
		msg := `╔═══════════════════════════╗
║    ❌ INVALID MODE         ║
╠═══════════════════════════╣
║  Valid modes:             ║
║  • public                 ║
║  • private                ║
║  • admin                  ║
╚═══════════════════════════╝`
		replyMessage(client, v, msg)
		return
	}

	s := getGroupSettings(v.Info.Chat.String())
	s.Mode = mode
	saveGroupSettings(s)

	var modeDesc string
	switch mode {
	case "public":
		modeDesc = "Everyone can use"
	case "private":
		modeDesc = "Bot disabled"
	case "admin":
		modeDesc = "Admin only"
	}

	msg := fmt.Sprintf(`╔═══════════════════════════╗
║   ✅ MODE CHANGED           ║
╠═══════════════════════════╣
║                           ║
║  🛡️ *New Mode:* %s        ║
║  📝 *Description:*        ║
║     %s                    ║
║                           ║
║  ✅ *Successfully Updated* ║
║                           ║
╚═══════════════════════════╝`, strings.ToUpper(mode), modeDesc)

	replyMessage(client, v, msg)
}

func handleReadAllStatus(client *whatsmeow.Client, v *events.Message) {
	if !isOwner(client, v.Info.Sender) {
		return
	}

	client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, time.Now(), types.NewJID("status@broadcast", types.DefaultUserServer), v.Info.Sender, types.ReceiptTypeRead)

	msg := `╔═══════════════════════════╗
║  ✅ STATUSES MARKED READ    ║
╠═══════════════════════════╣
║  All recent statuses have ║
║  been marked as read      ║
╚═══════════════════════════╝`

	replyMessage(client, v, msg)
}
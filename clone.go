package main

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// -------------------------
// Small helpers
// -------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func methodOnly(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, method+" only", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func botFromQuery(w http.ResponseWriter, r *http.Request) (*whatsmeow.Client, bool) {
	botID := r.URL.Query().Get("bot_id")
	if botID == "" {
		http.Error(w, "bot_id missing", http.StatusBadRequest)
		return nil, false
	}
	clientsMutex.RLock()
	bot, ok := activeClients[botID]
	clientsMutex.RUnlock()
	if !ok || bot == nil {
		http.Error(w, "Bot offline", http.StatusNotFound)
		return nil, false
	}
	return bot, true
}

// -------------------------
// Health / Version
// -------------------------

func handleHealthV2(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"ok":        true,
		"time":      time.Now().UTC().Format(time.RFC3339),
		"bots_live": func() int { clientsMutex.RLock(); defer clientsMutex.RUnlock(); return len(activeClients) }(),
	})
}

func handleVersionV2(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"service":  "impossible-bot",
		"go":       runtime.Version(),
		"platform": runtime.GOOS + "/" + runtime.GOARCH,
	})
}

// -------------------------
// Session/Auth (stubs for now)
// -------------------------

func handlePairV2(w http.ResponseWriter, r *http.Request) {
	// TODO: implement pairing via whatsmeow (same as handlePairAPI but JSON response)
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

func handleLogoutV2(w http.ResponseWriter, r *http.Request) {
	// TODO: bot logout / delete session
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

func handleSessionListV2(w http.ResponseWriter, r *http.Request) {
	// TODO: list sessions from sqlstore container
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

func handleSessionDeleteV2(w http.ResponseWriter, r *http.Request) {
	// TODO: delete a session from sqlstore + disconnect bot
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

// -------------------------
// Bots
// -------------------------

func handleBotListV2(w http.ResponseWriter, r *http.Request) {
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()

	out := make([]map[string]any, 0, len(activeClients))
	for id, c := range activeClients {
		isLogged := false
		if c != nil {
			isLogged = c.IsLoggedIn()
		}
		out = append(out, map[string]any{
			"bot_id":     id,
			"is_logged":  isLogged,
			"has_store":  c != nil && c.Store != nil,
			"store_id":   func() string { if c != nil && c.Store != nil && c.Store.ID != nil { return c.Store.ID.String() }; return "" }(),
		})
	}

	writeJSON(w, 200, map[string]any{"ok": true, "bots": out})
}

func handleBotOnlineV2(w http.ResponseWriter, r *http.Request) {
	_, ok := botFromQuery(w, r)
	if !ok {
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "online": true})
}

// -------------------------
// Chats (stubs)
// -------------------------

func handleChatsListV2(w http.ResponseWriter, r *http.Request) {
	// TODO: build from MySQL/Mongo history + store unread counts
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

func handleChatsSearchV2(w http.ResponseWriter, r *http.Request) {
	// TODO: search chats/messages in history DB
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

func handleChatOpenV2(w http.ResponseWriter, r *http.Request) {
	// optional analytics endpoint
	writeJSON(w, 200, map[string]any{"ok": true})
}

func handleChatMuteV2(w http.ResponseWriter, r *http.Request)   { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleChatPinV2(w http.ResponseWriter, r *http.Request)    { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleChatArchiveV2(w http.ResponseWriter, r *http.Request) { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleChatClearV2(w http.ResponseWriter, r *http.Request)   { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }

// -------------------------
// Messages (stubs)
// -------------------------

func handleMessagesHistoryV2(w http.ResponseWriter, r *http.Request) {
	// You already have handleGetHistoryV2.
	// Keep this for a separate schema if you want.
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

func handleMessageReplyV2(w http.ResponseWriter, r *http.Request)   { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleMessageForwardV2(w http.ResponseWriter, r *http.Request) { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleMessageReactV2(w http.ResponseWriter, r *http.Request)   { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleMessageStarV2(w http.ResponseWriter, r *http.Request)    { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleMessagePinV2(w http.ResponseWriter, r *http.Request)     { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleMessageDeleteV2(w http.ResponseWriter, r *http.Request)  { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleMessageEditV2(w http.ResponseWriter, r *http.Request)    { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleMessageReportV2(w http.ResponseWriter, r *http.Request)  { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }

// Delivery/Read receipts (standard)
func handleMarkDeliveredV2(w http.ResponseWriter, r *http.Request) { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleMarkReadV2(w http.ResponseWriter, r *http.Request)      { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }

// -------------------------
// Media (stubs)
// -------------------------

func handleMediaUploadV2(w http.ResponseWriter, r *http.Request) {
	// TODO: optional endpoint if frontend wants to upload then send reference
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

func handleMediaDownloadV2(w http.ResponseWriter, r *http.Request) {
	// TODO: proxy media download using whatsmeow direct path / URL + decrypt
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

func handleMediaThumbV2(w http.ResponseWriter, r *http.Request) {
	// TODO: generate thumbnails for UI
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

// -------------------------
// Contacts (stubs)
// -------------------------

func handleContactsListV2(w http.ResponseWriter, r *http.Request) {
	// TODO: list from bot.Store.Contacts (needs careful schema)
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

func handleContactBlockV2(w http.ResponseWriter, r *http.Request)   { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleContactUnblockV2(w http.ResponseWriter, r *http.Request) { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }

// -------------------------
// Profile & Settings (stubs except your existing update)
// -------------------------

func handleGetProfileV2(w http.ResponseWriter, r *http.Request) {
	bot, ok := botFromQuery(w, r)
	if !ok {
		return
	}
	// Basic profile info
	writeJSON(w, 200, map[string]any{
		"ok":        true,
		"bot_id":    r.URL.Query().Get("bot_id"),
		"is_logged": bot.IsLoggedIn(),
		"jid":       func() string { if bot.Store != nil && bot.Store.ID != nil { return bot.Store.ID.String() }; return "" }(),
	})
}

func handleProfilePrivacyV2(w http.ResponseWriter, r *http.Request) { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleGetSettingsV2(w http.ResponseWriter, r *http.Request)    { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleUpdateSettingsV2(w http.ResponseWriter, r *http.Request) { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }

// -------------------------
// Presence (stubs)
// -------------------------

func handlePresenceSubscribeV2(w http.ResponseWriter, r *http.Request)   { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handlePresenceUnsubscribeV2(w http.ResponseWriter, r *http.Request) { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handlePresenceSetV2(w http.ResponseWriter, r *http.Request)         { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }

// -------------------------
// Groups (stubs except your existing create/info/join/leave)
// -------------------------

func handleGroupsListV2(w http.ResponseWriter, r *http.Request) {
	// TODO: list groups from store or cached group list
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

func handleGroupMembersV2(w http.ResponseWriter, r *http.Request) {
	// Expected query: bot_id, group_id
	bot, ok := botFromQuery(w, r)
	if !ok {
		return
	}
	groupID := r.URL.Query().Get("group_id")
	if groupID == "" {
		http.Error(w, "group_id missing", http.StatusBadRequest)
		return
	}
	jid, err := types.ParseJID(groupID)
	if err != nil {
		http.Error(w, "invalid group_id", http.StatusBadRequest)
		return
	}

	info, err := bot.GetGroupInfo(context.Background(), jid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Basic member list (schema stable)
	members := make([]map[string]any, 0, len(info.Participants))
	for _, p := range info.Participants {
		members = append(members, map[string]any{
			"jid":      p.JID.String(),
			"is_admin": p.IsAdmin,
			"is_super": p.IsSuperAdmin,
		})
	}

	writeJSON(w, 200, map[string]any{"ok": true, "group_id": jid.String(), "members": members})
}

func handleGroupMemberAddV2(w http.ResponseWriter, r *http.Request)       { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleGroupMemberRemoveV2(w http.ResponseWriter, r *http.Request)    { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleGroupPromoteV2(w http.ResponseWriter, r *http.Request)         { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleGroupDemoteV2(w http.ResponseWriter, r *http.Request)          { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleGroupSetSubjectV2(w http.ResponseWriter, r *http.Request)      { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleGroupSetDescriptionV2(w http.ResponseWriter, r *http.Request)  { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleGroupSetPhotoV2(w http.ResponseWriter, r *http.Request)        { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleGroupInviteLinkV2(w http.ResponseWriter, r *http.Request)      { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }
func handleGroupInviteRevokeV2(w http.ResponseWriter, r *http.Request)    { writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"}) }

// -------------------------
// Status (stubs except your existing send/list)
// -------------------------

func handleStatusDeleteV2(w http.ResponseWriter, r *http.Request) {
	// TODO: delete status (depends on message revoke support)
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

func handleStatusViewersV2(w http.ResponseWriter, r *http.Request) {
	// TODO: viewer list retrieval (needs appstate / receipts)
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

func handleStatusMuteV2(w http.ResponseWriter, r *http.Request) {
	// TODO: mute status updates (local setting)
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

// -------------------------
// Webhook (stubs)
// -------------------------

func handleWebhookRegisterV2(w http.ResponseWriter, r *http.Request) {
	// TODO: store webhook URL + events in Redis/DB
	writeJSON(w, 501, map[string]any{"ok": false, "error": "not implemented yet"})
}

func handleWebhookTestV2(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"ok":     true,
		"event":  "test",
		"status": "delivered",
	})
}

// -------------------------
// NOTE: If you have any naming collisions, rename handlers in main accordingly.
// -------------------------

// (Optional) simple route guard
func rejectIfNoJSON(w http.ResponseWriter, r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return true
	}
	// allow common json types
	if strings.Contains(ct, "application/json") {
		return true
	}
	http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
	return false
}
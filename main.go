package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

var client *whatsmeow.Client

func main() {
	// سیشن کے لیے Postgres (ریلوے انوائرمنٹ سے)
	dbURL := os.Getenv("DATABASE_URL")
	container, _ := sqlstore.New("postgres", dbURL, waLog.Stdout("Database", "INFO", true))
	deviceStore, _ := container.GetFirstDevice()

	client = whatsmeow.NewClient(deviceStore, waLog.Stdout("Client", "INFO", true))
	client.AddEventHandler(eventHandler)

	// Gin ویب سرور
	r := gin.Default()
	r.StaticFile("/", "./web/index.html")
	r.StaticFile("/pic.png", "./web/pic.png")
	
	r.POST("/api/pair", func(c *gin.Context) {
		var req struct{ Number string `json:"number"` }
		c.BindJSON(&req)
		// پیرنگ لاجک
		client.Connect()
		code, _ := client.PairPhone(req.Number, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
		c.JSON(200, gin.H{"code": code})
	})
	
	go r.Run(":8080")
	client.Connect()

	// Exit signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	client.Disconnect()
}

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		body := v.Message.GetConversation()
		if body == "#menu" {
			sendOfficialMenu(v.Info.Chat)
		}
	}
}

// واٹس ایپ 3-Line مینیو بٹن
func sendOfficialMenu(chat types.JID) {
	listMsg := &waProto.ListMessage{
		Title:       proto.String("IMPOSSIBLE BOT"),
		Description: proto.String("Select category from the menu below"),
		ButtonText:  proto.String("MENU"), // آپ کی فرمائش پر نام "MENU" کر دیا گیا ہے
		ListType:    waProto.ListMessage_SINGLE_SELECT.Enum(),
		Sections: []*waProto.ListSection{
			{Title: proto.String("ADMIN TOOLS"), Rows: []*waProto.ListRow{{Title: proto.String("Kick"), RowId: proto.String("kick")}, {Title: proto.String("Add"), RowId: proto.String("add")}}},
			{Title: proto.String("DOWNLOADERS"), Rows: []*waProto.ListRow{{Title: proto.String("IG"), RowId: proto.String("ig")}, {Title: proto.String("TikTok"), RowId: proto.String("tt")}}},
			{Title: proto.String("SETTINGS"), Rows: []*waProto.ListRow{{Title: proto.String("Always Online"), RowId: proto.String("online")}}},
		},
	}
	client.SendMessage(context.Background(), chat, &waProto.Message{ListMessage: listMsg})
}
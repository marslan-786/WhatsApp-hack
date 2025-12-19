package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
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
	fmt.Println("🚀 Initializing Impossible Bot Engine...")

	// ڈیٹا بیس سیٹ اپ
	dbURL := os.Getenv("DATABASE_URL")
	dbType := "postgres"
	if dbURL == "" {
		dbURL = "file:impossible_session.db?_foreign_keys=on"
		dbType = "sqlite3"
	}

	dbLog := waLog.Stdout("Database", "INFO", true)
	// فکسڈ: context.Background() شامل کر دیا گیا ہے
	container, err := sqlstore.New(context.Background(), dbType, dbURL, dbLog)
	if err != nil {
		fmt.Printf("❌ Database Init Error: %v\n", err)
		panic(err)
	}

	// فکسڈ: context.Background() شامل کر دیا گیا ہے
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		panic(err)
	}

	client = whatsmeow.NewClient(deviceStore, waLog.Stdout("Client", "INFO", true))
	client.AddEventHandler(eventHandler)

	// ویب سرور (Gin)
	port := os.Getenv("PORT")
	if port == "" { port = "8080" }
	
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.StaticFile("/", "./web/index.html")
	r.StaticFile("/pic.png", "./web/pic.png")

	r.POST("/api/pair", func(c *gin.Context) {
		var req struct{ Number string `json:"number"` }
		if err := c.BindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid input"})
			return
		}

		if !client.IsConnected() {
			client.Connect()
		}

		// پیرنگ لاجک
		code, err := client.PairPhone(context.Background(), req.Number, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": code})
	})

	go func() {
		fmt.Printf("🌐 Web Dashboard live on port %s\n", port)
		r.Run(":" + port)
	}()

	if client.Store.ID != nil {
		client.Connect()
	}

	// Exit signal handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	client.Disconnect()
}

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		body := v.Message.GetConversation()
		if body == "" {
			body = v.Message.GetExtendedTextMessage().GetText()
		}

		// مینیو کمانڈ چیک کرنا
		if strings.TrimSpace(body) == "#menu" {
			sendOfficialMenu(v.Info.Chat)
		}
	}
}

// واٹس ایپ MENU بٹن (List Message)
func sendOfficialMenu(chat types.JID) {
	listMsg := &waProto.ListMessage{
		Title:       proto.String("IMPOSSIBLE BOT MENU"),
		Description: proto.String("Select a command category below"),
		ButtonText:  proto.String("MENU"),
		ListType:    waProto.ListMessage_SINGLE_SELECT.Enum(),
		Sections: []*waProto.ListMessage_Section{
			{
				Title: proto.String("AVAILABLE TOOLS"),
				Rows: []*waProto.ListMessage_Row{
					{Title: proto.String("Ping Status"), RowID: proto.String("ping")},
					{Title: proto.String("Check ID"), RowID: proto.String("id")},
				},
			},
		},
	}
	client.SendMessage(context.Background(), chat, &waProto.Message{ListMessage: listMsg})
}
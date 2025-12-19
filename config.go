package main

type ConfigStruct struct {
	OwnerName     string
	OwnerNumber   string
	BotName       string
	MongoURI      string
	Prefix        string
}

var Config = ConfigStruct{
	OwnerName:   "Nothing Is Impossible 🜲", //
	OwnerNumber: "923027665767",              //
	BotName:     "Group Guard",               //
	MongoURI:    "mongodb://mongo:AEvrikOWlrmJCQrDTQgfGtqLlwhwLuAA@crossover.proxy.rlwy.net:29609", //
	Prefix:       "#",
}
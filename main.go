package main

import (
	"gateway/server"
	"github.com/cuigh/auxo/app"
	"github.com/cuigh/auxo/app/flag"
	"github.com/cuigh/auxo/config"
)

func main() {
	config.SetProfile("dev")
	app.Name = "knot-gateway"
	app.Version = "0.1"
	app.Desc = "knot websocket gateway"
	app.Flags.Register(flag.All)
	app.Action = buildServer
	app.Start()
}

func buildServer(_ *app.Context) (err error) {
	ws := server.NewServer()
	app.Run(ws)
	return
}

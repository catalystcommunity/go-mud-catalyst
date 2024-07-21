package main

import (
	"log"
	"os"

	"github.com/catalystcommunity/muddycore/pkg/client"
	"github.com/catalystcommunity/muddycore/pkg/server"
	"github.com/urfave/cli/v2"
)

func runClient() {
	c := client.NewClient()
	c.Run()
}

func runServer() {
	s := server.NewServer("", "7777")
	s.StartServer()
}

func main() {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:    "client",
				Aliases: []string{"s"},
				Usage:   "client connect",
				Action: func(cCtx *cli.Context) error {
					runClient()
					return nil
				},
			},
			{
				Name:    "serve",
				Aliases: []string{"s"},
				Usage:   "serve stuff",
				Action: func(cCtx *cli.Context) error {
					runServer()
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

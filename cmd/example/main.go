package main

import (
	"context"
	"fmt"
	"log"
	"os"

	urfaveclimcp "github.com/thepwagner/urfave-cli-mcp"
	"github.com/urfave/cli/v3"
)

var App = &cli.Command{
	Name:  "example",
	Usage: "example",
	Commands: []*cli.Command{
		{
			Name:  "hello",
			Usage: "say hello",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "name",
					Usage: "the name to say hello to",
					Value: "World",
				},
			},
			Action: func(_ context.Context, c *cli.Command) error {
				fmt.Println("Hello,", c.String("name"))
				return nil
			},
		},
		{
			Name:  "add",
			Usage: "Calculate the sum of two numbers",
			Flags: []cli.Flag{
				&cli.Int64Flag{
					Name:  "first",
					Usage: "the first number to add",
				},
				&cli.Int64Flag{
					Name:  "second",
					Usage: "the second number to add",
				},
			},
			Action: func(_ context.Context, c *cli.Command) error {
				first := c.Int64("first")
				second := c.Int64("second")
				fmt.Printf("%d + %d = %d\n", first, second, first+second)
				return nil
			},
		},
	},
}

func main() {
	App.Commands = append(App.Commands, urfaveclimcp.MCPCommand(App))

	if err := App.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

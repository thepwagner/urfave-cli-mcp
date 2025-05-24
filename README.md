# urfave-cli-mcp

This is a quick hack for reusing a [urfave/cli](https://github.com/urfave/cli) app as an Model Context Protocol server.
It crawls the `Command` tree provided and exposes subcommands as MCP tools. Supports descriptions, flags, default values, and required flags.
Tools are invoked by forking the current process, and returning stdout as the result.

I'm using it to reuse query-only CLIs as an MCP server with one line of code.
It might also be useful to create new MCP servers, as you can iterate tools as a CLI before exposing to an MCP client.

### Example


```golang
package main

import (
    "context"
    "fmt"
    "github.com/urfave/cli/v3"
    urfaveclimcp "github.com/thepwagner/urfave-cli-mcp"
)

var App = &cli.Command{
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
        fmt.Println("Hello, ", c.String("name"))
        return nil
    },
}

func main() {
    // Pass the root command you want to expose, then add the "mcp" command to your App
    cli.App.Commands = append(cli.App.Commands, urfaveclimcp.NewMCPCommand(cli.App))
    cli.App.Run(context.Background(), os.Args)
}

```


package urfaveclimcp_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	urfaveclimcp "github.com/thepwagner/urfave-cli-mcp"
	"github.com/urfave/cli/v3"
)

func init() {
	// The server is going to fork os.Args[0], because it thinks that is the CLI app.
	// Right now it's actually some test harness value, let's spoof it to a known ~safe value
	os.Args = []string{"/bin/echo"}
}

func TestMCPCommand(t *testing.T) {
	t.Parallel()

	root := &cli.Command{
		Name:   "test",
		Action: func(context.Context, *cli.Command) error { return nil },
	}

	cmd := urfaveclimcp.MCPCommand(root)
	assert.Equal(t, "mcp", cmd.Name)
	assert.NotEmpty(t, cmd.Usage)
}

func TestMCPCommandServer(t *testing.T) {
	t.Parallel()

	root := &cli.Command{
		Name:    "test",
		Usage:   "do a test",
		Version: "1.0.0",
		Action:  func(context.Context, *cli.Command) error { return nil },
		Commands: []*cli.Command{
			{
				Name:        "sub",
				Description: "do a sub test",
				Flags: []cli.Flag{
					&cli.Int64Flag{
						Name:     "target",
						Usage:    "submarine to target",
						Value:    688,
						Required: true,
					},
					&cli.BoolFlag{
						Name:   "hidden",
						Usage:  "hidden flag",
						Hidden: true,
					},
				},
				Action: func(context.Context, *cli.Command) error { return nil },
			},
		},
	}
	srv, err := urfaveclimcp.MPCServer(root, true)
	assert.NoError(t, err)

	transport := transport.NewInProcessTransport(srv)
	c := client.NewClient(transport)

	initResult, err := c.Initialize(t.Context(), mcp.InitializeRequest{})
	require.NoError(t, err)
	assert.Equal(t, "test", initResult.ServerInfo.Name)
	assert.Equal(t, "1.0.0", initResult.ServerInfo.Version)
	assert.NotNil(t, initResult.Capabilities.Tools)
	assert.Nil(t, initResult.Capabilities.Resources)
	assert.Nil(t, initResult.Capabilities.Prompts)
	assert.Nil(t, initResult.Capabilities.Logging)
	assert.Nil(t, initResult.Capabilities.Experimental)

	tools, err := c.ListTools(t.Context(), mcp.ListToolsRequest{})
	require.NoError(t, err)
	if assert.Len(t, tools.Tools, 2) {
		assert.Equal(t, "test", tools.Tools[0].Name)
		assert.Equal(t, "do a test", tools.Tools[0].Description)
		assert.Empty(t, tools.Tools[0].InputSchema.Properties)
		assert.Empty(t, tools.Tools[0].InputSchema.Required)

		assert.Equal(t, "test_sub", tools.Tools[1].Name)
		assert.Equal(t, "do a sub test", tools.Tools[1].Description)
		assert.Equal(t, map[string]any{
			"type":        "number",
			"description": "submarine to target",
			"default":     float64(688),
		}, tools.Tools[1].InputSchema.Properties["target"])
		assert.Equal(t, []string{"target"}, tools.Tools[1].InputSchema.Required)
	}
}

func TestMCPCommandServer_CallTool(t *testing.T) {
	t.Parallel()

	root := &cli.Command{
		Name: "test",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "target"},
		},
		Action: func(context.Context, *cli.Command) error {
			// This is not actually executed, but must exist.
			return nil
		},
	}
	srv, err := urfaveclimcp.MPCServer(root, true)
	assert.NoError(t, err)

	transport := transport.NewInProcessTransport(srv)
	c := client.NewClient(transport)
	_, err = c.Initialize(t.Context(), mcp.InitializeRequest{})
	require.NoError(t, err)

	req := mcp.CallToolRequest{}
	req.Params.Name = "test"
	req.Params.Arguments = map[string]any{"target": "689"}
	callResult, err := c.CallTool(t.Context(), req)
	require.NoError(t, err)
	assert.Len(t, callResult.Content, 1)
	content, ok := callResult.Content[0].(mcp.TextContent)
	assert.True(t, ok)

	// The output of `echo --target 689` is returned, because that's how we would call the CLI app.
	assert.Equal(t, "--target 689\n", content.Text)
}

func TestMCPCommandServer_CallTool_Subcommand(t *testing.T) {
	t.Parallel()

	root := &cli.Command{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name:   "sub",
				Action: func(context.Context, *cli.Command) error { return nil },
			},
		},
	}
	srv, err := urfaveclimcp.MPCServer(root, false, "foo", "bar")
	assert.NoError(t, err)

	transport := transport.NewInProcessTransport(srv)
	c := client.NewClient(transport)
	_, err = c.Initialize(t.Context(), mcp.InitializeRequest{})
	require.NoError(t, err)

	req := mcp.CallToolRequest{}
	req.Params.Name = strings.Join([]string{"test", "sub"}, urfaveclimcp.ToolDelimiter)

	callResult, err := c.CallTool(t.Context(), req)
	require.NoError(t, err)
	assert.Len(t, callResult.Content, 1)
	content, ok := callResult.Content[0].(mcp.TextContent)
	assert.True(t, ok)

	assert.Equal(t, "foo bar sub\n", content.Text)
}

func TestMCPCommandServer_IgnoresHiddenCommandsAndSubcommands(t *testing.T) {
	t.Parallel()

	root := &cli.Command{
		Name:   "test",
		Action: func(context.Context, *cli.Command) error { return nil },
		Commands: []*cli.Command{
			{
				Name:   "visible",
				Usage:  "a visible command",
				Action: func(context.Context, *cli.Command) error { return nil },
			},
			{
				Name:   "mcp",
				Usage:  "should be hidden",
				Action: func(context.Context, *cli.Command) error { return nil },
			},
			{
				Name:   "hidden",
				Usage:  "should be hidden",
				Hidden: true,
				Action: func(context.Context, *cli.Command) error { return nil },
			},
			{
				Name:   "help",
				Usage:  "should be hidden",
				Action: func(context.Context, *cli.Command) error { return nil },
			},
			{
				Name:   "parent",
				Usage:  "parent with hidden subcommands",
				Action: func(context.Context, *cli.Command) error { return nil },
				Commands: []*cli.Command{
					{
						Name:   "visible-sub",
						Usage:  "a visible subcommand",
						Action: func(context.Context, *cli.Command) error { return nil },
					},
					{
						Name:   "mcp",
						Action: func(context.Context, *cli.Command) error { return nil },
					},
					{
						Name:   "hidden",
						Usage:  "hidden subcommand",
						Hidden: true,
						Action: func(context.Context, *cli.Command) error { return nil },
					},
					{
						Name:   "help",
						Usage:  "hidden subcommand",
						Action: func(context.Context, *cli.Command) error { return nil },
					},
				},
			},
		},
	}

	srv, err := urfaveclimcp.MPCServer(root, true)
	assert.NoError(t, err)

	transport := transport.NewInProcessTransport(srv)
	c := client.NewClient(transport)
	_, err = c.Initialize(t.Context(), mcp.InitializeRequest{})
	require.NoError(t, err)

	tools, err := c.ListTools(t.Context(), mcp.ListToolsRequest{})
	require.NoError(t, err)

	expectedTools := []string{"test", "test_visible", "test_parent", "test_parent_visible-sub"}

	assert.Len(t, tools.Tools, len(expectedTools))
	toolNames := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		toolNames = append(toolNames, tool.Name)
	}

	for _, expected := range expectedTools {
		assert.Contains(t, toolNames, expected)
	}
}

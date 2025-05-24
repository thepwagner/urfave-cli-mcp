package urfaveclimcp

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/urfave/cli/v3"
)

const ToolDelimiter = "_"

func MCPCommand(root *cli.Command, prefix ...string) *cli.Command {
	// Calling root.Run will modify root.Action, so store if it is non-nil first
	// We'll use this to decide to hide the root app if it's just a help command.
	hasRootAction := root.Action != nil

	return &cli.Command{
		Name:  "mcp",
		Usage: "Serve commands as MCP server on stdio",
		Action: func(ctx context.Context, _ *cli.Command) error {
			slog.Debug("building MCP server", slog.Any("app", root.Name))

			slog.Debug("serving MCP server")
			srv, err := MPCServer(root, hasRootAction, prefix...)
			if err != nil {
				return err
			}
			s := server.NewStdioServer(srv)
			return s.Listen(ctx, os.Stdin, os.Stdout)
		},
	}
}

func MPCServer(root *cli.Command, hasRootAction bool, prefix ...string) (*server.MCPServer, error) {
	srv := server.NewMCPServer(root.Name, root.Version, server.WithToolCapabilities(true))

	toolHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := strings.Split(request.Params.Name, ToolDelimiter)

		// Assume we were called with a root command - it should not be forwarded when we fork.
		args = args[1:]

		// If we were not called from the root command, add any prefix that was specified:
		args = append(prefix, args...)

		// We're about to execute some user input, how bad of an idea is that?
		// Because we hardcode `os.Args[0]` and don't use a shell, we're safe from command injection.
		// I _assume_ mcp-go has verified the arguments are actually what our tool said it can use.
		// The biggest risk is probably that we invoke ourselves, we can guard against that:
		if slices.Contains(args, "mcp") {
			return nil, fmt.Errorf("cannot invoke MCP from MCP, dawg")
		}

		for key, val := range request.GetArguments() {
			k := fmt.Sprintf("--%s", key)
			switch v := val.(type) {
			case string:
				args = append(args, k, v)
			case bool:
				args = append(args, k, strconv.FormatBool(v))
			case float64:
				// TODO: differentiate floats from ints? maybe just round?
				args = append(args, k, strconv.FormatFloat(v, 'f', -1, 64))
			}
		}
		var logFields []any
		for i, arg := range args {
			logFields = append(logFields, slog.Any(fmt.Sprintf("%d", i), arg))
		}
		slog.Info("forking", slog.Any("cmd", os.Args[0]), slog.Group("args", logFields...))

		var stdout, stderr bytes.Buffer
		p := exec.CommandContext(ctx, os.Args[0], args...)
		p.Stdout = &stdout
		p.Stderr = &stderr
		slog.Debug("invoked tool", slog.String("stderr", stderr.String()))
		if err := p.Run(); err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{mcp.NewTextContent(stderr.String())},
			}, nil
		}

		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(stdout.String())}}, nil
	}

	// Recurse the command and register as tools:
	var register func(cmd *cli.Command, prefix ...string) error
	register = func(cmd *cli.Command, prefix ...string) error {
		if cmd.Name == "mcp" || cmd.Name == "help" {
			return nil
		}

		loc := append(prefix, cmd.Name)
		if cmd.Action != nil && (len(prefix) > 0 || hasRootAction) {
			slog.Debug("registering command", slog.Any("loc", loc))
			toolOpts, err := FlagsToTools(cmd.Flags)
			if err != nil {
				return fmt.Errorf("failed to convert flags to tools %s: %w", loc, err)
			}

			var desc string
			if cmd.Description != "" {
				desc = cmd.Description
			} else {
				desc = cmd.Usage
			}

			toolOpts = append(toolOpts, mcp.WithDescription(desc))
			toolName := strings.Join(loc, "_")
			t := mcp.NewTool(toolName, toolOpts...)

			srv.AddTool(t, toolHandler)
		}
		for _, sub := range cmd.Commands {
			if err := register(sub, loc...); err != nil {
				return err
			}
		}
		return nil
	}
	if err := register(root); err != nil {
		return nil, err
	}

	return srv, nil
}

func FlagsToTools(flags []cli.Flag) ([]mcp.ToolOption, error) {
	var opts []mcp.ToolOption
	for _, flag := range flags {
		switch f := flag.(type) {
		case *cli.StringFlag:
			propOpts := []mcp.PropertyOption{
				mcp.Description(f.Usage),
			}
			if f.Required {
				propOpts = append(propOpts, mcp.Required())
			}
			if f.Value != "" {
				propOpts = append(propOpts, mcp.DefaultString(f.Value))
			}
			opts = append(opts, mcp.WithString(f.Name, propOpts...))

		case *cli.BoolFlag:
			if f.Name == "help" {
				continue
			}
			propOpts := []mcp.PropertyOption{
				mcp.Description(f.Usage),
				mcp.DefaultBool(f.Value),
			}
			if f.Required {
				propOpts = append(propOpts, mcp.Required())
			}
			opts = append(opts, mcp.WithBoolean(f.Name, propOpts...))

		// Numbers are nearly identical.
		case *cli.IntFlag:
			opts = append(opts, numberToolOption(f.Name, f.Usage, f.Value, f.Required))
		case *cli.Int8Flag:
			opts = append(opts, numberToolOption(f.Name, f.Usage, f.Value, f.Required))
		case *cli.Int16Flag:
			opts = append(opts, numberToolOption(f.Name, f.Usage, f.Value, f.Required))
		case *cli.Int32Flag:
			opts = append(opts, numberToolOption(f.Name, f.Usage, f.Value, f.Required))
		case *cli.Int64Flag:
			opts = append(opts, numberToolOption(f.Name, f.Usage, f.Value, f.Required))
		case *cli.UintFlag:
			opts = append(opts, numberToolOption(f.Name, f.Usage, f.Value, f.Required))
		case *cli.Uint8Flag:
			opts = append(opts, numberToolOption(f.Name, f.Usage, f.Value, f.Required))
		case *cli.Uint16Flag:
			opts = append(opts, numberToolOption(f.Name, f.Usage, f.Value, f.Required))
		case *cli.Uint32Flag:
			opts = append(opts, numberToolOption(f.Name, f.Usage, f.Value, f.Required))
		case *cli.Uint64Flag:
			opts = append(opts, numberToolOption(f.Name, f.Usage, f.Value, f.Required))
		case *cli.Float32Flag:
			opts = append(opts, numberToolOption(f.Name, f.Usage, f.Value, f.Required))
		case *cli.Float64Flag:
			opts = append(opts, numberToolOption(f.Name, f.Usage, f.Value, f.Required))

		// TODO: slices?

		default:
			return nil, fmt.Errorf("unsupported flag type: %T", f)
		}
	}
	return opts, nil
}

func numberToolOption[T int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | float32 | float64](name, usage string, value T, required bool) mcp.ToolOption {
	propOpts := []mcp.PropertyOption{
		mcp.Description(usage),
		mcp.DefaultNumber(float64(value)),
	}
	if required {
		propOpts = append(propOpts, mcp.Required())
	}
	return mcp.WithNumber(name, propOpts...)
}

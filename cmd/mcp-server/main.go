// Package main implements the MCP server for the todo-log plugin.
//
// This server provides tools for managing a PostgreSQL container and
// executing queries against it. Communicates via stdio JSON-RPC
// (Model Context Protocol).
package main

import (
	"log"
	"os"

	"github.com/JamesPrial/todo-log/internal/mcpserver"
	"github.com/mark3labs/mcp-go/server"
)

func run() int {
	errLogger := log.New(os.Stderr, "[mcp-server] ", log.LstdFlags)

	srv, err := mcpserver.NewServer()
	if err != nil {
		errLogger.Printf("Failed to create MCP server: %v", err)
		return 1
	}

	if err := server.ServeStdio(srv, server.WithErrorLogger(errLogger)); err != nil {
		errLogger.Printf("Server error: %v", err)
		return 1
	}

	return 0
}

func main() {
	os.Exit(run())
}

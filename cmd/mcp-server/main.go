package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	mcp_srv "github.com/mark3labs/mcp-go/server"

	"iec104-sim/internal/mcp"
)

func main() {
	simulatorURL := flag.String("simulator", "http://localhost:8989",
		"IEC104 模拟器的 HTTP API 地址")
	mode := flag.String("mode", "both",
		"MCP 服务模式: instance / data / both")
	listen := flag.String("listen", "",
		"HTTP 监听地址 (如 :8081)。设置后以 HTTP SSE 模式启动，否则默认 stdio")
	flag.Parse()

	client := mcp.NewSimulatorClient(*simulatorURL)

	buildServer := func() *mcp_srv.MCPServer {
		switch *mode {
		case "instance":
			return mcp.NewInstanceManagerServer(client)
		case "data":
			return mcp.NewDataInterfaceServer(client)
		case "both":
			s := mcp_srv.NewMCPServer(
				"IEC104 Simulator MCP",
				"1.0.0",
				mcp_srv.WithLogging(),
			)
			instSrv := mcp.NewInstanceManagerServer(client)
			for _, t := range instSrv.ListTools() {
				s.AddTool(t.Tool, t.Handler)
			}
			dataSrv := mcp.NewDataInterfaceServer(client)
			for _, t := range dataSrv.ListTools() {
				s.AddTool(t.Tool, t.Handler)
			}
			return s
		default:
			fmt.Fprintf(os.Stderr, "未知模式: %s (可选: instance / data / both)\n", *mode)
			os.Exit(1)
			return nil
		}
	}

	s := buildServer()
	if s == nil {
		os.Exit(1)
	}

	if *listen != "" {
		log.Printf("启动 IEC104 Simulator MCP Server (HTTP SSE on %s)", *listen)
		log.Printf("连接模拟器: %s", *simulatorURL)
		sseServer := mcp_srv.NewSSEServer(s)
		if err := sseServer.Start(*listen); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	} else {
		log.Printf("启动 IEC104 Simulator MCP Server (stdio)")
		log.Printf("连接模拟器: %s", *simulatorURL)
		if err := mcp_srv.ServeStdio(s); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}
}

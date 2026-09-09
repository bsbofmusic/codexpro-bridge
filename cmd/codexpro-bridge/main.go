package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/codexpro/bridge/core"
	"github.com/codexpro/bridge/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	config, err := core.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	mcpServer, caps, err := server.Build(config)
	if err != nil {
		log.Fatal(err)
	}
	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return mcpServer },
		&mcp.StreamableHTTPOptions{
			JSONResponse:                 true,
			Stateless:                    true,
			DisableLocalhostProtection:   true,
			PropagateRequestCancellation: true,
		},
	)
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler:           server.Auth(mux, config, caps.HealthResponse),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 * 1024,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("codexpro-bridge version=%s listen=%s", server.RuntimeVersion, httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

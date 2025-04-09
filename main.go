package main

import (
	"context"
	"log/slog"
	"os"
)

func main() {
	serv := NewServer(":8081", ":8082")
	if err := serv.Run(context.Background()); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	slog.Info("Server stopped gracefully.")
}

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	steam "github.com/slava-go-dev/go-steamworks-pure"
)

type SimpleLogger struct {
	Logger *log.Logger
}

func (l *SimpleLogger) Infof(msg string, args ...interface{}) {
	l.Logger.Printf(msg, args...)
}

func (l *SimpleLogger) Debugf(msg string, args ...interface{}) {
	l.Logger.Printf(msg, args...)
}

func (l *SimpleLogger) Errorf(msg string, args ...interface{}) {
	l.Logger.Printf(msg, args...)
}

func (l *SimpleLogger) Warnf(msg string, args ...interface{}) {
	l.Logger.Printf(msg, args...)
}

func main() {
	logger := SimpleLogger{
		Logger: log.Default(),
	}
	executablePath := os.Args[0]
	executableDir := filepath.Dir(executablePath)
	steamClient, err := steam.New(480, executableDir, &logger)
	if err != nil {
		log.Fatal("Failed to initialize Steam:", err)
	}
	defer steamClient.Shutdown()

	if !steamClient.IsSteamInitialized() {
		log.Fatal("Steam not initialized or user not logged in")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	steamClient.StartCallbackDispatcher(ctx)

	steamID := steamClient.GetSteamID()
	steamName := steamClient.GetSteamName()
	language := steamClient.GetSteamLanguage()

	fmt.Printf("Steam ID: %d\n", steamID)
	fmt.Printf("Steam Name: %s\n", steamName)
	fmt.Printf("Language: %s\n", language)

	ticket, err := steamClient.GetAuthTicket(ctx, []byte("test"), 4*time.Second)
	if err != nil {
		log.Fatal("Failed to get auth ticket:", err)
	}

	fmt.Printf("Auth ticket received (size: %d bytes)\n", len(ticket))

	time.Sleep(5 * time.Second)
}

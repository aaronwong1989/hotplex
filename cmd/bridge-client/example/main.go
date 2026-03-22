//go:build ignore

// Example DingTalk adapter using the BridgeClient SDK.
// Run with:
//   HOTPLEX_BRIDGE_TOKEN=secret ./dingtalk-example
//
// Prerequisites:
//   - HotPlex BridgeServer running on port 8080 (bridge_port=8080, bridge_token=secret in config)
//   - DingTalk bot webhook endpoint forwarding to this binary
package main

import (
	"context"
	"log"
	"os"

	"github.com/hrygo/hotplex/cmd/bridge-client"
)

func main() {
	token := os.Getenv("HOTPLEX_BRIDGE_TOKEN")
	if token == "" {
		log.Fatal("HOTPLEX_BRIDGE_TOKEN environment variable is required")
	}

	client, err := bridgeclient.New(
		bridgeclient.URL("ws://localhost:8080/bridge"),
		bridgeclient.Platform("dingtalk"),
		bridgeclient.AuthToken(token),
		bridgeclient.Capabilities(
			bridgeclient.CapText,
			bridgeclient.CapCard,
			bridgeclient.CapButtons,
			bridgeclient.CapTyping,
		),
	)
	if err != nil {
		log.Fatalf("create bridge client: %v", err)
	}

	client.OnMessage(func(msg *bridgeclient.Message) *bridgeclient.Reply {
		// Log inbound message
		log.Printf("[dingtalk] session=%s user=%s room=%s content=%q",
			msg.SessionKey,
			msg.Metadata.UserID,
			msg.Metadata.RoomID,
			truncate(msg.Content, 100),
		)

		// Send typing indicator
		ctx := context.Background()
		client.Typing(ctx, msg.SessionKey)

		// Process the message (replace with actual DingTalk API call)
		reply := processMessage(msg)

		return reply
	})

	client.OnEvent(func(evt *bridgeclient.Event) {
		log.Printf("[dingtalk] event=%s", evt.Event)
	})

	log.Println("[dingtalk] connecting to BridgeServer ws://localhost:8080/bridge ...")
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("connect: %v", err)
	}
	log.Println("[dingtalk] connected, waiting for messages ...")

	// Block until context is cancelled
	<-ctx.Done()
	log.Println("[dingtalk] shutting down")
	client.Close()
}

// processMessage handles a DingTalk inbound message and returns a reply.
func processMessage(msg *bridgeclient.Message) *bridgeclient.Reply {
	// TODO: Integrate with DingTalk API
	// 1. Call DingTalk API to get user info from msg.Metadata.UserID
	// 2. Forward content to AI engine
	// 3. Return AI response as a Reply

	// Placeholder: echo back
	return &bridgeclient.Reply{
		Content:    "DingTalk adapter is connected via BridgeServer. Message received!",
		SessionKey: msg.SessionKey,
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

package relaycmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/hrygo/hotplex/internal/relay"
	"github.com/spf13/cobra"
)

// SessionCmd is the parent command for relay subcommands.
var SessionCmd = &cobra.Command{
	Use:   "relay",
	Short: "Bot-to-Bot Relay management commands",
}

func init() {
	// Register flags for each subcommand before adding them.
	addBindingCmd.Flags().String("chat-id", "", "Chat/Channel ID for the binding")
	addBindingCmd.Flags().String("platform", "", "Platform name (e.g., slack)")
	addBindingCmd.Flags().StringToString("bot", nil, "Bot name:URL pair (repeatable)")

	delBindingCmd.Flags().String("chat-id", "", "Chat/Channel ID of the binding to delete")

	testRelayCmd.Flags().String("to", "", "Target agent name")
	testRelayCmd.Flags().String("content", "ping", "Message content to send")

	SessionCmd.AddCommand(listBindingsCmd, addBindingCmd, delBindingCmd, testRelayCmd)
}

// ---------------------------------------------------------------------------
// list_bindings
// ---------------------------------------------------------------------------

var listBindingsCmd = &cobra.Command{
	Use:   "list_bindings",
	Short: "List all relay bindings",
	Args:  cobra.NoArgs,
	RunE:  runListBindings,
}

func runListBindings(_ *cobra.Command, _ []string) error {
	store := relay.NewBindingStore("")
	bindings := store.List()
	if len(bindings) == 0 {
		fmt.Println("No relay bindings found.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "PLATFORM\tCHAT ID\tBOTS"); err != nil {
		return err
	}
	for _, b := range bindings {
		botList := make([]string, 0, len(b.Bots))
		for name, url := range b.Bots {
			botList = append(botList, name+":"+url)
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", b.Platform, b.ChatID, strings.Join(botList, ", ")); err != nil {
			return err
		}
	}
	return w.Flush()
}

// ---------------------------------------------------------------------------
// add_binding
// ---------------------------------------------------------------------------

var addBindingCmd = &cobra.Command{
	Use:   "add_binding --chat-id <id> --platform <name> --bot <name:url> [--bot <name:url> ...]",
	Short: "Add a relay binding",
	Args:  cobra.NoArgs,
	RunE:  runAddBinding,
}

func runAddBinding(cmd *cobra.Command, _ []string) error {
	chatID, _ := cmd.Flags().GetString("chat-id")
	platform, _ := cmd.Flags().GetString("platform")
	bots, _ := cmd.Flags().GetStringToString("bot")

	if chatID == "" || platform == "" || len(bots) == 0 {
		return fmt.Errorf("--chat-id, --platform, and at least one --bot are required")
	}

	store := relay.NewBindingStore("")
	binding := &relay.RelayBinding{
		Platform: platform,
		ChatID:   chatID,
		Bots:     bots,
	}
	if err := store.Add(binding); err != nil {
		return fmt.Errorf("add binding: %w", err)
	}
	fmt.Printf("Relay binding added: %s:%s\n", platform, chatID)
	return nil
}

// ---------------------------------------------------------------------------
// del_binding
// ---------------------------------------------------------------------------

var delBindingCmd = &cobra.Command{
	Use:   "del_binding --chat-id <id>",
	Short: "Delete a relay binding by chat ID",
	Args:  cobra.NoArgs,
	RunE:  runDelBinding,
}

func runDelBinding(cmd *cobra.Command, _ []string) error {
	chatID, _ := cmd.Flags().GetString("chat-id")
	if chatID == "" {
		return fmt.Errorf("--chat-id is required")
	}
	store := relay.NewBindingStore("")
	if err := store.Delete(chatID); err != nil {
		return fmt.Errorf("delete binding: %w", err)
	}
	fmt.Printf("Relay binding deleted: %s\n", chatID)
	return nil
}

// ---------------------------------------------------------------------------
// test_relay
// ---------------------------------------------------------------------------

var testRelayCmd = &cobra.Command{
	Use:   "test_relay --to <agent-name> [--content <message>]",
	Short: "Send a test relay message to another agent",
	Args:  cobra.NoArgs,
	RunE:  runTestRelay,
}

func runTestRelay(cmd *cobra.Command, _ []string) error {
	to, _ := cmd.Flags().GetString("to")
	content, _ := cmd.Flags().GetString("content")

	if to == "" {
		return fmt.Errorf("--to is required")
	}

	// TODO: implement actual relay send once RelayClient is wired into Engine (Phase 3).
	// For now, verify agent exists in any binding.
	store := relay.NewBindingStore("")
	bindings := store.List()
	found := false
	for _, b := range bindings {
		if _, ok := b.Bots[to]; ok {
			found = true
			break
		}
	}
	if !found {
		fmt.Printf("Warning: agent %q not found in any binding.\n", to)
	}
	fmt.Printf("Sending relay to %s: %s\n", to, content)
	return nil
}

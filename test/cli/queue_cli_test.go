//go:build cli

package cli

import (
	"fmt"
	"testing"
)

func TestQueueLifecycle(t *testing.T) {
	requireQueue(t)
	name := uniqueName("cli-queue")

	az(t, "storage", "queue", "create", "--name", name, "--output", "none")

	if got := az(t, "storage", "queue", "exists", "--name", name, "--query", "exists", "-o", "tsv"); got != "true" {
		t.Fatalf("queue exists = %q, want true", got)
	}
	if got := az(t, "storage", "queue", "list", "--query", fmt.Sprintf("[?name=='%s'] | length(@)", name), "-o", "tsv"); got != "1" {
		t.Fatalf("queue listing count = %q, want 1", got)
	}

	az(t, "storage", "queue", "delete", "--name", name, "--output", "none")
}

func TestQueueMessageRoundTrip(t *testing.T) {
	requireQueue(t)
	name := uniqueName("cli-msg")
	az(t, "storage", "queue", "create", "--name", name, "--output", "none")
	t.Cleanup(func() {
		az(t, "storage", "queue", "delete", "--name", name, "--output", "none")
	})

	const content = "hello-from-the-cli"
	az(t, "storage", "message", "put", "--queue-name", name, "--content", content, "--output", "none")

	if got := az(t, "storage", "message", "peek", "--queue-name", name, "--query", "[0].content", "-o", "tsv"); got != content {
		t.Fatalf("peeked content = %q, want %q", got, content)
	}

	if got := az(t, "storage", "message", "get", "--queue-name", name, "--query", "[0].content", "-o", "tsv"); got != content {
		t.Fatalf("dequeued content = %q, want %q", got, content)
	}

	// Clearing removes any remaining messages.
	az(t, "storage", "message", "clear", "--queue-name", name, "--output", "none")
}

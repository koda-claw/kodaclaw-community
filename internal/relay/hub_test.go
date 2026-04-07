package relay

import "testing"

func TestHubOnEventNotConnected(t *testing.T) {
	hub := NewHub()

	result := hub.OnEvent("missing", EventFrame{EventID: "evt-1"})
	if result != DeliveryNotConnected {
		t.Fatalf("expected not connected, got %s", result)
	}
}

func TestHubOnEventQueued(t *testing.T) {
	hub := NewHub()
	client := NewClient(hub, nil)
	hub.Register("acct-1", client)

	frame := EventFrame{EventID: "evt-1"}
	result := hub.OnEvent("acct-1", frame)
	if result != DeliveryQueued {
		t.Fatalf("expected queued, got %s", result)
	}

	select {
	case queued := <-client.outCh:
		got, ok := queued.(EventFrame)
		if !ok {
			t.Fatalf("expected EventFrame, got %T", queued)
		}
		if got.EventID != "evt-1" {
			t.Fatalf("unexpected event id: %q", got.EventID)
		}
	default:
		t.Fatal("expected frame to be queued")
	}
}

func TestHubOnEventClientBusy(t *testing.T) {
	hub := NewHub()
	client := NewClient(hub, nil)
	hub.Register("acct-1", client)

	for i := 0; i < cap(client.outCh); i++ {
		client.outCh <- EventFrame{EventID: "prefill"}
	}

	result := hub.OnEvent("acct-1", EventFrame{EventID: "evt-1"})
	if result != DeliveryClientBusy {
		t.Fatalf("expected client busy, got %s", result)
	}
}

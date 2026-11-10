package services

import (
	"path/filepath"
	"testing"

	"github.com/WINGS-N/wingsv-dex/internal/config"
)

func TestAddInvalidProxyDoesNotCreateSubscription(t *testing.T) {
	store, err := config.NewStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatal(err)
	}
	before := len(store.SubscriptionList())
	service := NewSubscriptionService(store)
	if _, err := service.Add("Test", "https://example.test/sub", nil, "http://example.com:8080"); err == nil {
		t.Fatal("Add accepted a non-loopback proxy")
	}
	if got := len(store.SubscriptionList()); got != before {
		t.Fatalf("subscription count = %d, want unchanged at %d", got, before)
	}
}

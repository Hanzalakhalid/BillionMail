package antivirus

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveClamd(t *testing.T) {
	address := os.Getenv("BM_TEST_CLAMD_ADDR")
	if address == "" {
		t.Skip("BM_TEST_CLAMD_ADDR is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	scanner := Clamd{Address: address, Timeout: 10 * time.Second}
	if err := scanner.Health(ctx); err != nil {
		t.Fatal(err)
	}
	if err := scanner.Scan(ctx, strings.NewReader("A harmless product reference.")); err != nil {
		t.Fatalf("clean stream rejected: %v", err)
	}
	// The standard benign EICAR antivirus fixture is decoded only in container
	// memory. It is never written as an executable or placed in the repository.
	fixture, err := base64.StdEncoding.DecodeString("WDVPIVAlQEFQWzRcUFpYNTQoUF4pN0NDKTd9JEVJQ0FSLVNUQU5EQVJELUFOVElWSVJVUy1URVNULUZJTEUhJEgrSCo=")
	if err != nil || len(fixture) != 68 {
		t.Fatal("invalid antivirus test fixture")
	}
	if err := scanner.Scan(ctx, bytes.NewReader(fixture)); !errors.Is(err, ErrInfected) {
		t.Fatalf("EICAR detection result=%v", err)
	}
}

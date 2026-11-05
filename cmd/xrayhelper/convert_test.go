package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

func TestConvertStdoutJSON(t *testing.T) {
	if os.Getenv("WINGSV_CONVERT_TEST_CHILD") == "1" {
		if err := cmdConvert(); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestConvertStdoutJSON$")
	cmd.Env = append(os.Environ(), "WINGSV_CONVERT_TEST_CHILD=1")
	cmd.Stdin = bytes.NewBufferString("vless://00000000-0000-0000-0000-000000000001@example.invalid:443?encryption=none&security=tls&type=ws&path=%2F&host=example.invalid&sni=example.invalid&alpn=h2&fp=chrome#Test")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected conversion log output on stderr")
	}
	var result struct {
		Outbounds []json.RawMessage `json:"outbounds"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("convert stdout is not JSON: %v", err)
	}
	if len(result.Outbounds) != 1 {
		t.Fatalf("converted outbounds = %d, want 1", len(result.Outbounds))
	}
}

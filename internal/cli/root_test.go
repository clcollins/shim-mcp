package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	Version = "1.2.3"
	Commit = "abc1234"
	BuildDate = "2026-06-08T00:00:00Z"

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "1.2.3") {
		t.Errorf("output missing version: %q", output)
	}
	if !strings.Contains(output, "abc1234") {
		t.Errorf("output missing commit: %q", output)
	}
}

func TestRootCommand_ConfigFlag(t *testing.T) {
	rootCmd.SetArgs([]string{"--config", "/tmp/test-config.yaml", "version"})
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("root command error: %v", err)
	}

	if cfgFile != "/tmp/test-config.yaml" {
		t.Errorf("cfgFile = %q, want /tmp/test-config.yaml", cfgFile)
	}
}

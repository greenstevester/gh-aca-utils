package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Test command creation and basic structure
func TestCmdIPPort_Creation(t *testing.T) {
	cmd := cmdIPPort()

	if cmd.Use != "ip-port" {
		t.Errorf("Expected command use to be 'ip-port', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected command to have a short description")
	}

	// Check that required flags exist
	repoFlag := cmd.Flags().Lookup("repo")
	if repoFlag == nil {
		t.Error("Expected --repo flag to exist")
	}

	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("Expected --output flag to exist")
	}
}

func TestCmdFlipAdapters_Creation(t *testing.T) {
	cmd := cmdFlipAdapters()

	if cmd.Use != "flip-adapters" {
		t.Errorf("Expected command use to be 'flip-adapters', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected command to have a short description")
	}

	// Check that required flags exist
	requiredFlags := []string{"repo", "env", "adapters"}
	for _, flagName := range requiredFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected --%s flag to exist", flagName)
		}
	}

	// Check boolean flags
	boolFlags := []string{"commit", "pr", "dry-run"}
	for _, flagName := range boolFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected --%s flag to exist", flagName)
			continue
		}
		if flag.Value.Type() != "bool" {
			t.Errorf("Expected --%s to be boolean flag", flagName)
		}
	}
}

// Test the root command structure
func TestExecute_RootCommand(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer

	// Create root command
	root := &cobra.Command{Use: "aca", Short: "IP/Port extraction + adapter toggler"}
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.AddCommand(cmdIPPort())
	root.AddCommand(cmdFlipAdapters())

	// Test that help works
	root.SetArgs([]string{"--help"})
	err := root.Execute()
	if err != nil {
		t.Errorf("Expected help command to succeed, got error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ip-port") {
		t.Error("Expected help output to contain 'ip-port' command")
	}
	if !strings.Contains(output, "flip-adapters") {
		t.Error("Expected help output to contain 'flip-adapters' command")
	}
}

// Test command validation
func TestCmdIPPort_Validation(t *testing.T) {
	cmd := cmdIPPort()

	// Set up command with no arguments
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test without required --repo flag
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when --repo flag is missing")
	}
	if !strings.Contains(err.Error(), "repo") {
		t.Errorf("Expected error message to mention 'repo', got: %v", err)
	}
}

func TestCmdFlipAdapters_Validation(t *testing.T) {
	cmd := cmdFlipAdapters()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test missing --repo
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when required flags are missing")
	}

	// Test missing --env
	cmd.SetArgs([]string{"--repo", "org/repo"})
	err = cmd.Execute()
	if err == nil {
		t.Error("Expected error when --env flag is missing")
	}
	if !strings.Contains(err.Error(), "env") {
		t.Errorf("Expected error message to mention 'env', got: %v", err)
	}

	// Test missing --adapters
	cmd.SetArgs([]string{"--repo", "org/repo", "--env", "dev"})
	err = cmd.Execute()
	if err == nil {
		t.Error("Expected error when --adapters flag is missing")
	}
	if !strings.Contains(err.Error(), "adapters") {
		t.Errorf("Expected error message to mention 'adapters', got: %v", err)
	}
}

// Test output mode parsing in commands
func TestOutputModeFlags(t *testing.T) {
	tests := []struct {
		flag     string
		expected outputMode
	}{
		{"csv", outCSV},
		{"table", outTable},
		{"json", outJSON},
		{"CSV", outCSV},
		{"TABLE", outTable},
		{"JSON", outJSON},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			result := parseMode(tt.flag, outTable)
			if result != tt.expected {
				t.Errorf("parseMode(%q) = %v, want %v", tt.flag, result, tt.expected)
			}
		})
	}
}

// Test flag default values
func TestCommandDefaults(t *testing.T) {
	// Test ip-port defaults
	ipCmd := cmdIPPort()

	includeFlag := ipCmd.Flags().Lookup("include")
	if includeFlag == nil {
		t.Fatal("include flag not found")
	}
	includeDefault := includeFlag.DefValue
	expectedIncludes := "**/*.properties,**/*.yml,**/*.yaml,**/*.conf,**/*.ini,**/*.txt,**/*.env,**/*.json"
	if includeDefault != expectedIncludes {
		t.Errorf("Expected include default to be %q, got %q", expectedIncludes, includeDefault)
	}

	excludeFlag := ipCmd.Flags().Lookup("exclude")
	if excludeFlag == nil {
		t.Fatal("exclude flag not found")
	}
	excludeDefault := excludeFlag.DefValue
	expectedExcludes := "**/.git/**,**/node_modules/**,**/dist/**"
	if excludeDefault != expectedExcludes {
		t.Errorf("Expected exclude default to be %q, got %q", expectedExcludes, excludeDefault)
	}

	// Test flip-adapters defaults
	flipCmd := cmdFlipAdapters()

	dryRunFlag := flipCmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Fatal("dry-run flag not found")
	}
	if dryRunFlag.DefValue != "true" {
		t.Errorf("Expected dry-run default to be 'true', got %q", dryRunFlag.DefValue)
	}

	outputFlag := flipCmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Fatal("output flag not found")
	}
	if outputFlag.DefValue != "table" {
		t.Errorf("Expected flip-adapters output default to be 'table', got %q", outputFlag.DefValue)
	}
}

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test pattern matching functionality
func TestMatchAny(t *testing.T) {
	tests := []struct {
		path     string
		patterns []string
		want     bool
	}{
		{"src/main.go", []string{"**/*.go"}, true},
		{"src/main.go", []string{"**/*.js"}, false},
		{"node_modules/pkg/file.js", []string{"**/node_modules/**"}, true},
		{"dist/bundle.js", []string{"**/dist/**", "**/build/**"}, true},
		{"config/app.properties", []string{"**/*.properties", "**/*.yml"}, true},
		{"README.md", []string{"**/*.go", "**/*.js"}, false},
		{"deep/nested/path/file.txt", []string{"**/*.txt"}, true},
		{".git/config", []string{"**/.git/**"}, true},
		{"src/.git/hooks/pre-commit", []string{"**/.git/**"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			// Normalize the test path for cross-platform compatibility
			normalizedPath := filepath.ToSlash(tt.path)
			got := matchAny(normalizedPath, tt.patterns)
			if got != tt.want {
				t.Errorf("matchAny(%q, %v) = %v, want %v", normalizedPath, tt.patterns, got, tt.want)
			}
		})
	}
}

// Test display width calculation
func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello", 5},
		{"", 0},
		{"café", 4}, // Unicode characters
		{"🚀", 1},    // Emoji (counts as 1 rune)
		{"hello world", 11},
		{"tab\there", 8},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := displayWidth(tt.input)
			if got != tt.want {
				t.Errorf("displayWidth(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// Test regex patterns
func TestRegexPatterns(t *testing.T) {
	// Test IPv4 pattern
	ipv4Tests := []struct {
		input string
		want  bool
	}{
		{"192.168.1.1", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"10.0.0.1", true},
		{"192.168.1.256", false}, // Invalid octet
		{"192.168.1", false},     // Incomplete
		{"hello 192.168.1.1 world", true},
		{"no ip here", false},
	}

	for _, tt := range ipv4Tests {
		t.Run("IPv4_"+tt.input, func(t *testing.T) {
			got := ipv4.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("IPv4 pattern match for %q = %v, want %v", tt.input, got, tt.want)
			}
		})
	}

	// Test IPv6 pattern
	ipv6Tests := []struct {
		input string
		want  bool
	}{
		{"::1", true},
		{"2001:db8::1", true},
		{"fe80::1", true},
		{"2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"not:an:ipv6", false},
		{"hello ::1 world", true},
	}

	for _, tt := range ipv6Tests {
		t.Run("IPv6_"+tt.input, func(t *testing.T) {
			got := ipv6.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("IPv6 pattern match for %q = %v, want %v", tt.input, got, tt.want)
			}
		})
	}

	// Test key-value pattern
	kvTests := []struct {
		input string
		want  bool
	}{
		{"key=value", true},
		{"host.name = localhost", true},
		{"port: 8080", true},
		{"spaced_key = spaced value", true},
		{"invalid line", false},
		{"# comment = not a kv", false},
		{"key_with_underscores=value", true},
		{"key-with-dashes=value", true},
		{"key.with.dots=value", true},
	}

	for _, tt := range kvTests {
		t.Run("KV_"+tt.input, func(t *testing.T) {
			got := kvRe.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("KV pattern match for %q = %v, want %v", tt.input, got, tt.want)
			}
		})
	}

	// Test port pattern
	portTests := []struct {
		input string
		want  bool
	}{
		{"server_port: 8080", true},
		{"httpPort=3000", true},
		{"port \"8080\"", true},
		{"port '3000'", true},
		{"timeout: 30", false},  // Too short for port range
		{"port: 999999", false}, // Too long for port range
		{"not a port line", false},
		{"database.port = 5432", true},
	}

	for _, tt := range portTests {
		t.Run("Port_"+tt.input, func(t *testing.T) {
			got := portRe.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("Port pattern match for %q = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// Test temporary directory and file operations
func TestTempOperations(t *testing.T) {
	// Test that temp directory creation works
	tmpDir := t.TempDir()

	// Verify directory exists
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Errorf("Temp directory should exist: %v", err)
	}

	// Test file creation in temp directory
	testFile := filepath.Join(tmpDir, "test.properties")
	content := "test.key=test.value\nserver.port=8080\n"

	if err := os.WriteFile(testFile, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Verify file exists and has correct content
	readContent, err := os.ReadFile(testFile) // #nosec G304 -- testFile is safely constructed in test
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	if string(readContent) != content {
		t.Errorf("File content mismatch. Expected %q, got %q", content, string(readContent))
	}
}

// Test edge cases in string processing
func TestStringEdgeCases(t *testing.T) {
	// Test empty and whitespace strings
	emptyTests := []string{"", "   ", "\t", "\n", "\r\n"}

	for _, input := range emptyTests {
		t.Run("empty_"+input, func(t *testing.T) {
			key, val, ok := parseKV(input)
			if ok {
				t.Errorf("parseKV(%q) should return false for empty/whitespace, got key=%q, val=%q",
					input, key, val)
			}
		})
	}

	// Test very long strings
	longKey := strings.Repeat("a", 1000)
	longVal := strings.Repeat("b", 1000)
	longLine := longKey + "=" + longVal

	key, val, ok := parseKV(longLine)
	if !ok {
		t.Error("parseKV should handle long strings")
	}
	if key != longKey || strings.TrimSpace(val) != longVal {
		t.Error("parseKV should correctly parse long strings")
	}

	// Test special characters
	specialChars := []string{
		"key.with.dots=value",
		"key_with_underscores=value",
		"key-with-dashes=value",
		"key123=value456",
		"UPPERCASE_KEY=UPPERCASE_VALUE",
	}

	for _, line := range specialChars {
		t.Run("special_"+line, func(t *testing.T) {
			_, _, ok := parseKV(line)
			if !ok {
				t.Errorf("parseKV should handle line with special chars: %q", line)
			}
		})
	}
}

// Test comment detection edge cases
func TestCommentDetection(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"# Normal comment", true},
		{"; Semicolon comment", true},
		{"   # Indented comment", true},
		{"\t# Tab indented comment", true},
		{"key=value # Not a comment line", false},
		{"#key=value", true}, // Still a comment even if it looks like kv
		{";key=value", true}, // Still a comment even if it looks like kv
		{"##", true},         // Double hash is still comment
		{";;", true},         // Double semicolon is still comment
		{"# ", true},         // Comment with space
		{"; ", true},         // Comment with space
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isCommentOrBlank(tt.input)
			if got != tt.want {
				t.Errorf("isCommentOrBlank(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// Test boundary conditions for port validation
func TestPortBoundaryConditions(t *testing.T) {
	tests := []struct {
		key   string
		value string
		want  bool
	}{
		{"port", "22", true},            // Minimum valid port (2 digits)
		{"port", "65535", true},         // Maximum port number
		{"port", "1", false},            // Too short (less than 2 digits)
		{"port", "123456", false},       // Too long (more than 5 digits)
		{"port", "80", true},            // Common port
		{"port", "443", true},           // Common port
		{"port", "8080", true},          // Common port
		{"httpPort", "3000", true},      // Port in key name
		{"database_port", "5432", true}, // Port in key name with underscore
		{"timeout", "5000", false},      // Not a port key
		{"PORT", "8080", true},          // Uppercase port key
	}

	for _, tt := range tests {
		t.Run(tt.key+"_"+tt.value, func(t *testing.T) {
			got := looksLikePort(tt.key, tt.value)
			if got != tt.want {
				t.Errorf("looksLikePort(%q, %q) = %v, want %v", tt.key, tt.value, got, tt.want)
			}
		})
	}
}

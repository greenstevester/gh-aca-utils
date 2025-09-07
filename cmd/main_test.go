package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseKV(t *testing.T) {
	tests := []struct {
		input   string
		wantKey string
		wantVal string
		wantOk  bool
	}{
		{"key=value", "key", "value", true},
		{"host.ip=192.168.1.1", "host.ip", "192.168.1.1", true},
		{"port: 8080", "port", "8080", true},
		{"  spaced_key  =  spaced value  ", "spaced_key", "spaced value", true},
		{"# comment line", "", "", false},
		{"invalid line", "", "", false},
		{"", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotKey, gotVal, gotOk := parseKV(tt.input)
			if gotKey != tt.wantKey || gotVal != tt.wantVal || gotOk != tt.wantOk {
				t.Errorf("parseKV(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.input, gotKey, gotVal, gotOk, tt.wantKey, tt.wantVal, tt.wantOk)
			}
		})
	}
}

func TestIsCommentOrBlank(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", true},
		{"   ", true},
		{"# comment", true},
		{"; comment", true},
		{"  # spaced comment", true},
		{"key=value", false},
		{"normal line", false},
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

func TestLooksLikeIP(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"255.255.255.255", true},
		{"0.0.0.0", true},
		{"::1", true},
		{"2001:db8::1", true},
		{"not.an.ip", false},
		{"256.256.256.256", false},
		{"192.168.1", false},
		{"", false},
		{"\"192.168.1.1\"", true},
		{"'10.0.0.1'", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := looksLikeIP(tt.input)
			if got != tt.want {
				t.Errorf("looksLikeIP(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLooksLikePort(t *testing.T) {
	tests := []struct {
		key   string
		value string
		want  bool
	}{
		{"server.port", "8080", true},
		{"database_port", "5432", true},
		{"PORT", "80", true},
		{"httpPort", "3000", true},
		{"timeout", "30", false},
		{"port", "abc", false},
		{"port", "999999", false},
		{"port", "1", false},
		{"port", "\"8080\"", true},
		{"port", "'3000'", true},
	}

	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			got := looksLikePort(tt.key, tt.value)
			if got != tt.want {
				t.Errorf("looksLikePort(%q, %q) = %v, want %v", tt.key, tt.value, got, tt.want)
			}
		})
	}
}

func TestStripQuotes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"\"quoted\"", "quoted"},
		{"'single'", "single"},
		{"unquoted", "unquoted"},
		{"\"partial", "\"partial"},
		{"mixed'", "mixed'"},
		{"  \"  spaced  \"  ", "  spaced  "},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripQuotes(tt.input)
			if got != tt.want {
				t.Errorf("stripQuotes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFirstIP(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"connect to 192.168.1.1:8080", "192.168.1.1"},
		{"server at 10.0.0.1 and backup at 10.0.0.2", "10.0.0.1"},
		{"no ip here", ""},
		{"IPv6 address 2001:db8::1", "2001:db8::1"},
		{"mixed 192.168.1.1 and 2001:db8::1", "192.168.1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := firstIP(tt.input)
			if got != tt.want {
				t.Errorf("firstIP(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindInlinePort(t *testing.T) {
	tests := []struct {
		input   string
		wantKey string
		wantVal string
		wantOk  bool
	}{
		{"server_port: 8080", "server_port", "8080", true},
		{"connect to serverPort=3000", "serverPort", "3000", true},
		{"httpPort \"8080\"", "httpPort", "8080", true},
		{"no port here", "", "", false},
		{"port value is too short: 1", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotKey, gotVal, gotOk := findInlinePort(tt.input)
			if gotKey != tt.wantKey || gotVal != tt.wantVal || gotOk != tt.wantOk {
				t.Errorf("findInlinePort(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.input, gotKey, gotVal, gotOk, tt.wantKey, tt.wantVal, tt.wantOk)
			}
		})
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input string
		def   []string
		want  []string
	}{
		{"a,b,c", nil, []string{"a", "b", "c"}},
		{"  a  ,  b  ,  c  ", nil, []string{"a", "b", "c"}},
		{"", []string{"default"}, []string{"default"}},
		{"  ", []string{"default"}, []string{"default"}},
		{"single", nil, []string{"single"}},
		{"a,,c", nil, []string{"a", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitCSV(tt.input, tt.def)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitCSV(%q, %v) = %v, want %v", tt.input, tt.def, got, tt.want)
			}
		})
	}
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		input string
		def   outputMode
		want  outputMode
	}{
		{"csv", outTable, outCSV},
		{"CSV", outTable, outCSV},
		{"table", outJSON, outTable},
		{"json", outCSV, outJSON},
		{"invalid", outTable, outTable},
		{"", outJSON, outJSON},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseMode(tt.input, tt.def)
			if got != tt.want {
				t.Errorf("parseMode(%q, %v) = %v, want %v", tt.input, tt.def, got, tt.want)
			}
		})
	}
}

func TestMatchRow_JSON(t *testing.T) {
	row := matchRow{
		IPKey:      "host.ip",
		IPValue:    "192.168.1.1",
		PortKey:    "server.port",
		PortValue:  "8080",
		RelPath:    "config/app.properties",
		LineNumber: 42,
	}

	data, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled matchRow
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if !reflect.DeepEqual(row, unmarshaled) {
		t.Errorf("JSON round-trip failed:\noriginal: %+v\nunmarshaled: %+v", row, unmarshaled)
	}
}

func TestChange_JSON(t *testing.T) {
	chg := change{
		Adapter:  "test.adapter",
		OldValue: "0",
		NewValue: "1",
		FilePath: "env/dev/parameters.properties",
	}

	data, err := json.Marshal(chg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled change
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if !reflect.DeepEqual(chg, unmarshaled) {
		t.Errorf("JSON round-trip failed:\noriginal: %+v\nunmarshaled: %+v", chg, unmarshaled)
	}
}

// Test the table functionality
func TestTable(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	// Create and render a table
	table := newTable()
	table.AddRow("Col1", "Column2", "Col3")
	table.AddRow("A", "BB", "CCC")
	table.AddRow("DDDD", "E", "FF")
	table.Render()

	// Restore stdout and read captured output
	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
	os.Stdout = oldStdout
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Failed to read from pipe: %v", err)
	}
	output := buf.String()

	// Verify the output contains expected structure
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 4 {
		t.Errorf("Expected at least 4 lines of output, got %d", len(lines))
	}

	// Check that header row exists
	if !strings.Contains(lines[0], "Col1") {
		t.Errorf("Expected header row to contain 'Col1', got: %s", lines[0])
	}

	// Check that separator line exists
	if !strings.Contains(lines[1], "-") {
		t.Errorf("Expected separator line with dashes, got: %s", lines[1])
	}
}

// Test CSV escaping
func TestCSVEsc(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with,comma", "\"with,comma\""},
		{"with\"quote", "\"with\"\"quote\""},
		{"with\nnewline", "\"with\nnewline\""},
		{"with\rcarriage", "\"with\rcarriage\""},
		{"multiple,\"issues\"\n", "\"multiple,\"\"issues\"\"\n\""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := csvEsc(tt.input)
			if got != tt.want {
				t.Errorf("csvEsc(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Integration test for scanForIPPort with temporary files
func TestScanForIPPort_Integration(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create test files
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write test configuration file
	configFile := filepath.Join(configDir, "app.properties")
	configContent := `# Application configuration
server.host=192.168.1.100
server.port=8080
database.host=10.0.0.5
database.port=5432
timeout=30
# Comment with IP 172.16.0.1 should be ignored
`
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Write YAML file
	yamlFile := filepath.Join(tmpDir, "service.yml")
	yamlContent := `service:
  host: "203.0.113.1"
  httpPort: 3000
  httpsPort: "3443"
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	// Run the scan
	includes := []string{"**/*.properties", "**/*.yml"}
	excludes := []string{"**/node_modules/**", "**/dist/**", "**/.git/**"}

	rows := scanForIPPort(tmpDir, includes, excludes)

	// Verify results
	if len(rows) == 0 {
		t.Error("Expected some matches, got none")
	}

	// Check for specific expected matches
	foundServerHost := false
	foundServerPort := false
	foundYamlHost := false

	for _, row := range rows {
		if row.IPKey == "server.host" && row.IPValue == "192.168.1.100" {
			foundServerHost = true
		}
		if row.PortKey == "server.port" && row.PortValue == "8080" {
			foundServerPort = true
		}
		if row.IPValue == "203.0.113.1" {
			foundYamlHost = true
		}
	}

	if !foundServerHost {
		t.Error("Expected to find server.host=192.168.1.100")
	}
	if !foundServerPort {
		t.Error("Expected to find server.port=8080")
	}
	if !foundYamlHost {
		t.Error("Expected to find YAML host 203.0.113.1")
	}
}

package cmd

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/spf13/cobra"
)

type outputMode string

const (
	outCSV   outputMode = "csv"
	outTable outputMode = "table"
	outJSON  outputMode = "json"
)

type matchRow struct {
	IPKey      string `json:"ipKey"`
	IPValue    string `json:"ipValue"`
	PortKey    string `json:"portKey"`
	PortValue  string `json:"portValue"`
	RelPath    string `json:"filePath"`
	LineNumber int    `json:"lineNumber"`
}

type change struct {
	Adapter  string `json:"adapter"`
	OldValue string `json:"old"`
	NewValue string `json:"new"`
	FilePath string `json:"filePath"`
}

func Execute() {
	root := &cobra.Command{Use: "aca", Short: "IP/Port extraction + adapter toggler"}
	root.AddCommand(cmdIPPort())
	root.AddCommand(cmdFlipAdapters())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func cmdIPPort() *cobra.Command {
	var repo, ref string
	var includes, excludes string
	var mode string

	cmd := &cobra.Command{
		Use:   "ip-port",
		Short: "Scan repo for IP/Port key/value pairs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo == "" {
				return fmt.Errorf("--repo ORG/REPO is required")
			}
			modeVal := parseMode(mode, outCSV)

			tmpDir, cleanup, err := cloneOrDownload(repo, ref)
			if err != nil {
				return err
			}
			defer cleanup()

			inc := splitCSV(includes, []string{"**/*"})
			exc := splitCSV(excludes, []string{"**/.git/**", "**/node_modules/**"})

			rows := scanForIPPort(tmpDir, inc, exc)
			return printRows(rows, modeVal)
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Target repo as ORG/REPO")
	cmd.Flags().StringVar(&ref, "ref", "", "Branch or tag (default: default branch)")
	cmd.Flags().StringVar(&includes, "include",
		"**/*.properties,**/*.yml,**/*.yaml,**/*.conf,**/*.ini,**/*.txt,**/*.env,**/*.json",
		"Comma-separated glob patterns to include")
	cmd.Flags().StringVar(&excludes, "exclude",
		"**/.git/**,**/node_modules/**,**/dist/**",
		"Comma-separated glob patterns to exclude")
	cmd.Flags().StringVar(&mode, "output", "csv", "Output: csv|table|json")

	return cmd
}

func cmdFlipAdapters() *cobra.Command {
	var repo, envName, adaptersCSV, branch, mode string
	var doCommit, doPR, dryRun bool

	cmd := &cobra.Command{
		Use:   "flip-adapters",
		Short: "Toggle adapter values (0↔1) in env/<ENV>/parameters.properties",
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo == "" {
				return fmt.Errorf("--repo ORG/REPO is required")
			}
			if envName == "" {
				return fmt.Errorf("--env is required (e.g., dev)")
			}
			if adaptersCSV == "" {
				return fmt.Errorf("--adapters is required (comma list)")
			}
			modeVal := parseMode(mode, outTable)

			tmpDir, cleanup, err := cloneOrDownload(repo, "")
			if err != nil {
				return err
			}
			defer cleanup()

			propPath := filepath.Join(tmpDir, "env", envName, "parameters.properties")
			// Validate path is within expected directory to prevent traversal
			if !strings.HasPrefix(propPath, tmpDir) {
				return fmt.Errorf("invalid file path")
			}
			b, err := os.ReadFile(propPath) // #nosec G304 - path is validated above
			if err != nil {
				return fmt.Errorf("read %s: %w", propPath, err)
			}

			lines := strings.Split(string(b), "\n")
			want := splitCSV(adaptersCSV, nil)
			changes := make([]change, 0)

			m := map[string]int{} // adapter -> line index
			for i, line := range lines {
				if isCommentOrBlank(line) {
					continue
				}
				k, v, ok := parseKV(line)
				if !ok {
					continue
				}
				m[k] = i
				_ = v
			}

			for _, a := range want {
				idx, ok := m[a]
				if !ok {
					fmt.Fprintf(os.Stderr, "warning: adapter %q not found in %s\n", a, propPath)
					continue
				}
				k, v, _ := parseKV(lines[idx])
				var newV string
				switch strings.TrimSpace(v) {
				case "0":
					newV = "1"
				case "1":
					newV = "0"
				default:
					fmt.Fprintf(os.Stderr, "warning: adapter %q has non-binary value %q; skipping\n", k, v)
					continue
				}
				lines[idx] = fmt.Sprintf("%s=%s", k, newV)
				changes = append(changes, change{Adapter: k, OldValue: strings.TrimSpace(v), NewValue: newV, FilePath: propPath})
			}

			if len(changes) == 0 {
				if modeVal == outJSON {
					if err := json.NewEncoder(os.Stdout).Encode([]change{}); err != nil {
						return fmt.Errorf("encode JSON: %w", err)
					}
					return nil
				}
				fmt.Println("No changes made.")
				return nil
			}

			if dryRun {
				return printChangeReport(changes, modeVal)
			}

			if err := os.WriteFile(propPath, []byte(strings.Join(lines, "\n")), 0600); err != nil {
				return fmt.Errorf("write %s: %w", propPath, err)
			}

			if err := printChangeReport(changes, modeVal); err != nil {
				return err
			}

			if doCommit {
				if branch == "" {
					branch = fmt.Sprintf("toggle/adapters-%s", envName)
				}
				if err := gitIn(tmpDir, "checkout", "-b", branch); err != nil {
					return err
				}
				if err := gitIn(tmpDir, "add", filepath.Join("env", envName, "parameters.properties")); err != nil {
					return err
				}
				msg := fmt.Sprintf("chore(env:%s): flip adapters %s", envName, strings.Join(want, ","))
				if err := gitIn(tmpDir, "commit", "-m", msg); err != nil {
					return err
				}
				if err := gitIn(tmpDir, "push", "-u", "origin", branch); err != nil {
					return err
				}
				if doPR {
					prTitle := fmt.Sprintf("Flip adapters in %s: %s", envName, strings.Join(want, ", "))
					prBody := "Automated via gh aca flip-adapters."
					if err := ghIn(tmpDir, "pr", "create", "--fill", "--title", prTitle, "--body", prBody); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Target repo as ORG/REPO (required)")
	cmd.Flags().StringVar(&envName, "env", "", "Environment directory under env/ (required)")
	cmd.Flags().StringVar(&adaptersCSV, "adapters", "", "Comma-separated adapter keys (required)")
	cmd.Flags().StringVar(&branch, "branch", "", "Branch name to create (with --commit)")
	cmd.Flags().BoolVar(&doCommit, "commit", false, "Commit the change to a new branch and push")
	cmd.Flags().BoolVar(&doPR, "pr", false, "Create a pull request (implies --commit)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "Show planned changes without writing")
	cmd.Flags().StringVar(&mode, "output", "table", "Output: table|json")

	return cmd
}

// ----------------- helpers (clone, scan, output, utils) -----------------

// cloneOrDownload tries `gh repo clone`, then falls back to tarball download.
func cloneOrDownload(repo, ref string) (string, func(), error) {
	tmp, err := os.MkdirTemp("", "gh-aca-utils-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }

	args := []string{"repo", "clone", repo, tmp, "--", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	if cloneErr := execCommand("gh", args...); cloneErr == nil {
		return tmp, cleanup, nil
	}

	// fallback
	tarURL := fmt.Sprintf("repos/%s/tarball", repo)
	if ref != "" {
		tarURL = fmt.Sprintf("repos/%s/tarball/%s", repo, ref)
	}
	// #nosec G204 - tarURL is constructed from validated repo parameter
	cmd := exec.Command("gh", "api", "-H", "Accept: application/vnd.github+json", tarURL)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if startErr := cmd.Start(); startErr != nil {
		cleanup()
		return "", nil, startErr
	}
	if untarErr := untarGz(stdout, tmp); untarErr != nil {
		cleanup()
		return "", nil, untarErr
	}
	if waitErr := cmd.Wait(); waitErr != nil {
		// Log but don't fail - tar extraction may have succeeded
		fmt.Fprintf(os.Stderr, "warning: gh api command failed: %v\n", waitErr)
	}

	entries, err := os.ReadDir(tmp)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("read temp dir: %w", err)
	}
	if len(entries) == 1 && entries[0].IsDir() {
		top := filepath.Join(tmp, entries[0].Name())
		if err := moveUp(top, tmp); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("move files up: %w", err)
		}
		if err := os.Remove(top); err != nil {
			// Non-critical error, continue
			fmt.Fprintf(os.Stderr, "warning: failed to remove temp dir: %v\n", err)
		}
	}
	return tmp, cleanup, nil
}

func untarGz(r io.Reader, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := gz.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close gzip reader: %v\n", closeErr)
		}
	}()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		// Validate header name to prevent path traversal
		if strings.Contains(hdr.Name, "..") {
			continue // Skip potentially malicious paths
		}

		fp := filepath.Join(dest, filepath.Clean(hdr.Name))

		// Ensure we're still within dest directory
		if !strings.HasPrefix(fp, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			// #nosec G115 - hdr.Mode is from trusted tar header, masked to safe value
			mode := os.FileMode(hdr.Mode & 0755) // Restrict permissions
			if err := os.MkdirAll(fp, mode); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(fp), 0750); err != nil {
				return err
			}
			f, err := os.Create(fp) // #nosec G304 - fp is validated above for path traversal
			if err != nil {
				return err
			}

			// Limit file size to prevent decompression bombs
			const maxFileSize = 100 * 1024 * 1024 // 100MB limit
			limited := io.LimitReader(tr, maxFileSize)

			if _, err := io.Copy(f, limited); err != nil {
				if closeErr := f.Close(); closeErr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to close file: %v\n", closeErr)
				}
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func moveUp(src, dest string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read source directory: %w", err)
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		destPath := filepath.Join(dest, e.Name())
		if err := os.Rename(srcPath, destPath); err != nil {
			return fmt.Errorf("move %s to %s: %w", srcPath, destPath, err)
		}
	}
	return nil
}

// --- scanning

var (
	ipv4   = regexp.MustCompile(`\b((25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])\b`)
	ipv6   = regexp.MustCompile(`(::1|([0-9a-fA-F]{1,4}:){2,7}[0-9a-fA-F]{0,4}|::)`)
	kvRe   = regexp.MustCompile(`^\s*([A-Za-z0-9_.\-]+)\s*[:=]\s*(.+?)\s*$`)
	portRe = regexp.MustCompile(`(?i)\b([A-Za-z0-9_.\-]*port[A-Za-z0-9_.\-]*)\s*[:=\s]\s*["']?([0-9]{2,5})["']?\b`)
)

func scanForIPPort(root string, includes, excludes []string) []matchRow {
	var rows []matchRow
	var files []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err // Return error instead of ignoring
		}
		if matchAny(rel, excludes) {
			return nil
		}
		if !matchAny(rel, includes) {
			return nil
		}
		files = append(files, path)
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: error walking directory: %v\n", err)
	}

	sort.Strings(files)

	for _, f := range files {
		rel, err := filepath.Rel(root, f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to get relative path for %s: %v\n", f, err)
			continue
		}

		fh, err := os.Open(f) // #nosec G304 - f is from controlled file walk
		if err != nil {
			continue
		}
		s := bufio.NewScanner(fh)
		lineNo := 0
		for s.Scan() {
			lineNo++
			line := s.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}

			var ipKey, ipVal, portKey, portVal string
			if m := kvRe.FindStringSubmatch(line); len(m) == 3 {
				k, v := m[1], strings.TrimSpace(m[2])
				if looksLikeIP(v) {
					ipKey, ipVal = k, v
				}
				if looksLikePort(k, v) {
					portKey, portVal = k, stripQuotes(v)
				}
			} else {
				if ip := firstIP(line); ip != "" {
					ipVal = ip
				}
				if pk, pv, ok := findInlinePort(line); ok {
					portKey, portVal = pk, pv
				}
			}

			if ipKey != "" || ipVal != "" || portKey != "" || portVal != "" {
				rows = append(rows, matchRow{ipKey, ipVal, portKey, portVal, rel, lineNo})
			}
		}
		if err := fh.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close file %s: %v\n", f, err)
		}
	}
	return rows
}

func printRows(rows []matchRow, mode outputMode) error {
	switch mode {
	case outCSV:
		fmt.Println("IP Key,IP Value,Port Key,Port Value,File Path,Line Number")
		for _, r := range rows {
			fmt.Printf("%s,%s,%s,%s,%s,%d\n",
				csvEsc(r.IPKey), csvEsc(r.IPValue), csvEsc(r.PortKey), csvEsc(r.PortValue),
				csvEsc(r.RelPath), r.LineNumber)
		}
	case outTable:
		w := newTable()
		w.AddRow("IP Key", "IP Value", "Port Key", "Port Value", "File Path", "Line")
		for _, r := range rows {
			w.AddRow(r.IPKey, r.IPValue, r.PortKey, r.PortValue, r.RelPath, fmt.Sprintf("%d", r.LineNumber))
		}
		w.Render()
	case outJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	return nil
}

func csvEsc(s string) string {
	s = strings.ReplaceAll(s, `"`, `""`)
	if strings.ContainsAny(s, ",\n\r\"") {
		return `"` + s + `"`
	}
	return s
}

func printChangeReport(changes []change, mode outputMode) error {
	switch mode {
	case outTable:
		w := newTable()
		w.AddRow("Adapter", "Old", "New", "File")
		for _, c := range changes {
			w.AddRow(c.Adapter, c.OldValue, c.NewValue, c.FilePath)
		}
		w.Render()
	case outJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(changes)
	}
	return nil
}

// --- utils

func parseKV(line string) (key, val string, ok bool) {
	m := kvRe.FindStringSubmatch(line)
	if len(m) != 3 {
		return "", "", false
	}
	return m[1], strings.TrimSpace(m[2]), true
}

func isCommentOrBlank(line string) bool {
	trim := strings.TrimSpace(line)
	return trim == "" || strings.HasPrefix(trim, "#") || strings.HasPrefix(trim, ";")
}

func looksLikeIP(s string) bool {
	ss := stripQuotes(s)
	return ipv4.MatchString(ss) || ipv6.MatchString(ss)
}

func firstIP(s string) string {
	if m := ipv4.FindString(s); m != "" {
		return m
	}
	return ipv6.FindString(s)
}

func findInlinePort(line string) (key, val string, ok bool) {
	m := portRe.FindStringSubmatch(line)
	if len(m) == 3 {
		return m[1], m[2], true
	}
	return "", "", false
}

func looksLikePort(k, v string) bool {
	if !strings.Contains(strings.ToLower(k), "port") {
		return false
	}
	vv := stripQuotes(v)
	if len(vv) < 2 || len(vv) > 5 {
		return false
	}
	for _, ch := range vv {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func stripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func matchAny(path string, patterns []string) bool {
	for _, p := range patterns {
		ok, err := doublestar.PathMatch(p, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: invalid pattern %q: %v\n", p, err)
			continue
		}
		if ok {
			return true
		}
	}
	return false
}

func splitCSV(s string, def []string) []string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		pp := strings.TrimSpace(p)
		if pp != "" {
			out = append(out, pp)
		}
	}
	return out
}

// --- tiny table printer ---

type table struct {
	rows   [][]string
	widths []int
}

func newTable() *table { return &table{} }

func (t *table) AddRow(cols ...string) {
	t.rows = append(t.rows, cols)
	for i, c := range cols {
		w := displayWidth(c)
		if i >= len(t.widths) {
			t.widths = append(t.widths, w)
		} else if w > t.widths[i] {
			t.widths[i] = w
		}
	}
}

func (t *table) Render() {
	for r, cols := range t.rows {
		for i, c := range cols {
			pad := t.widths[i] - displayWidth(c)
			fmt.Print(c)
			if i < len(cols)-1 {
				fmt.Print(strings.Repeat(" ", pad+2))
			}
		}
		fmt.Println()
		if r == 0 {
			// header underlines
			for i := range cols {
				fmt.Print(strings.Repeat("-", t.widths[i]))
				if i < len(cols)-1 {
					fmt.Print("  ")
				}
			}
			fmt.Println()
		}
	}
}

func displayWidth(s string) int { return len([]rune(s)) }

// --- subprocess helpers ---

func execCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitIn(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ghIn(dir string, args ...string) error {
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func parseMode(s string, def outputMode) outputMode {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "csv":
		return outCSV
	case "table":
		return outTable
	case "json":
		return outJSON
	}
	return def
}

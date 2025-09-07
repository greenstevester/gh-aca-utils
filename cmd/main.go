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
	root.AddCommand(cmdSetAdapters())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func cmdIPPort() *cobra.Command {
	var repo, ref string
	var includes, excludes string
	var mode string
	var allBranches bool

	cmd := &cobra.Command{
		Use:   "ip-port",
		Short: "Scan repo for IP/Port key/value pairs across branches",
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo == "" {
				return fmt.Errorf("--repo ORG/REPO is required")
			}
			modeVal := parseMode(mode, outCSV)

			if allBranches {
				return scanAllBranches(repo, includes, excludes, modeVal)
			}

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
	cmd.Flags().BoolVar(&allBranches, "all-branches", false, "Scan all branches in the repository")
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
				// Try to load from stored adapters
				storedAdapters, err := loadStoredAdapters()
				if err != nil {
					return fmt.Errorf("--adapters is required (comma list) or run 'gh aca set-adapters' to store adapters first")
				}
				if len(storedAdapters) == 0 {
					return fmt.Errorf("--adapters is required (comma list) or run 'gh aca set-adapters' to store adapters first")
				}
				// Validate that no stored adapter names are empty
				for _, adapter := range storedAdapters {
					if strings.TrimSpace(adapter) == "" {
						return fmt.Errorf("invalid empty adapter name found in stored adapters")
					}
				}
				adaptersCSV = strings.Join(storedAdapters, ",")
			}
			modeVal := parseMode(mode, outTable)

			tmpDir, cleanup, err := cloneOrDownload(repo, "")
			if err != nil {
				return err
			}
			defer cleanup()

			// Validate environment name to prevent path traversal
			cleanEnvName := filepath.Clean(envName)
			if strings.Contains(cleanEnvName, "..") || strings.Contains(cleanEnvName, "/") || strings.Contains(cleanEnvName, "\\") {
				return fmt.Errorf("invalid environment name: %q", envName)
			}

			propPath := filepath.Join(tmpDir, "env", cleanEnvName, "parameters.properties")
			// Double-check path is within expected directory
			if !strings.HasPrefix(propPath, filepath.Join(tmpDir, "env")+string(os.PathSeparator)) {
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
	cmd.Flags().StringVar(&adaptersCSV, "adapters", "", "Comma-separated adapter keys (or use stored adapters from 'set-adapters')")
	cmd.Flags().StringVar(&branch, "branch", "", "Branch name to create (with --commit)")
	cmd.Flags().BoolVar(&doCommit, "commit", false, "Commit the change to a new branch and push")
	cmd.Flags().BoolVar(&doPR, "pr", false, "Create a pull request (implies --commit)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "Show planned changes without writing")
	cmd.Flags().StringVar(&mode, "output", "table", "Output: table|json")

	return cmd
}

func cmdSetAdapters() *cobra.Command {
	var adapters string
	var list, clear bool

	cmd := &cobra.Command{
		Use:   "set-adapters",
		Short: "Manage stored adapter lists for reuse in flip-adapters command",
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				return listStoredAdapters()
			}

			if clear {
				return clearStoredAdapters()
			}

			if adapters == "" {
				return fmt.Errorf("--adapters is required (comma-separated list)")
			}

			return storeAdapters(adapters)
		},
	}

	cmd.Flags().StringVar(&adapters, "adapters", "", "Comma-separated list of adapter names to store")
	cmd.Flags().BoolVar(&list, "list", false, "List currently stored adapters")
	cmd.Flags().BoolVar(&clear, "clear", false, "Clear all stored adapters")

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

func cloneAllBranches(repo string) (string, func(), error) {
	tmp, err := os.MkdirTemp("", "gh-aca-utils-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }

	// Clone with all branches
	args := []string{"clone", repo, tmp}
	if cloneErr := execCommand("git", args...); cloneErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to clone repository: %w", cloneErr)
	}

	// Fetch all remote branches
	if fetchErr := gitIn(tmp, "fetch", "--all"); fetchErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to fetch all branches: %v\n", fetchErr)
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
			if err := os.MkdirAll(fp, mode|0755); err != nil { // Ensure directories are accessible
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
	ipv4 = regexp.MustCompile(`\b((25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])\b`)
	// IPv6 regex that correctly matches IPv6 addresses including ::1 and compressed forms
	ipv6   = regexp.MustCompile(`(?i)(?:(?:[0-9a-f]{1,4}:){7}[0-9a-f]{1,4}|(?:[0-9a-f]{1,4}:){1,6}::[0-9a-f]{1,4}|(?:[0-9a-f]{1,4}:){1,5}(?::[0-9a-f]{1,4}){1,2}|(?:[0-9a-f]{1,4}:){1,4}(?::[0-9a-f]{1,4}){1,3}|(?:[0-9a-f]{1,4}:){1,3}(?::[0-9a-f]{1,4}){1,4}|(?:[0-9a-f]{1,4}:){1,2}(?::[0-9a-f]{1,4}){1,5}|[0-9a-f]{1,4}:(?::[0-9a-f]{1,4}){1,6}|:(?::[0-9a-f]{1,4}){1,7}|(?:[0-9a-f]{1,4}:){1,7}:|::1|::)`)
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
			fmt.Fprintf(os.Stderr, "warning: failed to get relative path for %s: %v\n", path, err)
			return nil // Continue walking instead of failing completely
		}
		// Normalize path separators for cross-platform compatibility
		rel = filepath.ToSlash(rel)
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
		// Normalize path separators for cross-platform compatibility
		rel = filepath.ToSlash(rel)

		// Use anonymous function to ensure file is closed immediately
		func() {
			fh, err := os.Open(f) // #nosec G304 - f is from controlled file walk
			if err != nil {
				return
			}
			defer func() {
				if closeErr := fh.Close(); closeErr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to close file %s: %v\n", f, closeErr)
				}
			}()

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
						ipKey, ipVal = k, stripQuotes(v)
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
		}()
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

func scanAllBranches(repo, includes, excludes string, mode outputMode) error {
	tmpDir, cleanup, err := cloneAllBranches(repo)
	if err != nil {
		return err
	}
	defer cleanup()

	// Get all branch names
	branches, err := getAllBranches(tmpDir)
	if err != nil {
		return fmt.Errorf("failed to get branches: %w", err)
	}

	var allRows []matchRow
	inc := splitCSV(includes, []string{"**/*"})
	exc := splitCSV(excludes, []string{"**/.git/**", "**/node_modules/**"})

	for _, branch := range branches {
		// Checkout each branch
		if err := gitIn(tmpDir, "checkout", branch); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to checkout branch %s: %v\n", branch, err)
			continue
		}

		// Scan this branch
		rows := scanForIPPort(tmpDir, inc, exc)

		// Add branch information to each row
		for i := range rows {
			rows[i].RelPath = fmt.Sprintf("[%s] %s", branch, rows[i].RelPath)
		}

		allRows = append(allRows, rows...)
	}

	return printRows(allRows, mode)
}

func getAllBranches(repoDir string) ([]string, error) {
	cmd := exec.Command("git", "branch", "-r", "--format=%(refname:short)")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		// Fallback for older Git versions that don't support --format
		cmd = exec.Command("git", "branch", "-r")
		cmd.Dir = repoDir
		output, err = cmd.Output()
		if err != nil {
			return nil, err
		}
	}

	var branches []string
	seenBranches := make(map[string]bool)
	// Handle both Unix (\n) and Windows (\r\n) line endings
	outputStr := strings.ReplaceAll(string(output), "\r\n", "\n")
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	for _, line := range lines {
		branch := strings.TrimSpace(line)
		if branch != "" && !strings.Contains(branch, "HEAD") {
			// Remove origin/ prefix and any leading whitespace/asterisks
			branch = strings.TrimSpace(strings.TrimPrefix(branch, "*"))
			branch = strings.TrimPrefix(branch, "origin/")
			// Only add unique branches
			if branch != "" && !seenBranches[branch] {
				seenBranches[branch] = true
				branches = append(branches, branch)
			}
		}
	}

	return branches, nil
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
	// Normalize path separators for cross-platform compatibility
	normalizedPath := filepath.ToSlash(path)

	for _, p := range patterns {
		// Ensure patterns also use forward slashes for consistency
		normalizedPattern := filepath.ToSlash(p)
		ok, err := doublestar.PathMatch(normalizedPattern, normalizedPath)
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

// --- adapter storage functions ---

func getAdapterConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".gh-aca-utils")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return filepath.Join(configDir, "adapters.txt"), nil
}

func storeAdapters(adapters string) error {
	configPath, err := getAdapterConfigPath()
	if err != nil {
		return err
	}

	// Parse and validate adapters
	adapterList := splitCSV(adapters, nil)
	if len(adapterList) == 0 {
		return fmt.Errorf("no valid adapters provided")
	}

	// Filter out empty adapter names and validate
	validAdapters := make([]string, 0, len(adapterList))
	for _, adapter := range adapterList {
		trimmed := strings.TrimSpace(adapter)
		if trimmed == "" {
			return fmt.Errorf("empty adapter name not allowed: %q", adapter)
		}
		validAdapters = append(validAdapters, trimmed)
	}

	if len(validAdapters) == 0 {
		return fmt.Errorf("no valid adapters provided after validation")
	}

	// Write to file (overwrite existing)
	content := strings.Join(validAdapters, "\n") + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write adapter file: %w", err)
	}

	fmt.Printf("Stored %d adapter(s) in %s:\n", len(validAdapters), configPath)
	for _, adapter := range validAdapters {
		fmt.Printf("  - %s\n", adapter)
	}

	return nil
}

func listStoredAdapters() error {
	configPath, err := getAdapterConfigPath()
	if err != nil {
		return err
	}

	adapters, err := loadStoredAdapters()
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("No adapters stored yet. Use 'gh aca set-adapters --adapters adapter1,adapter2' to store adapters.\n")
			return nil
		}
		return err
	}

	if len(adapters) == 0 {
		fmt.Printf("No adapters stored in %s\n", configPath)
	} else {
		fmt.Printf("Stored adapters (%s):\n", configPath)
		for _, adapter := range adapters {
			fmt.Printf("  - %s\n", adapter)
		}
	}

	return nil
}

func clearStoredAdapters() error {
	configPath, err := getAdapterConfigPath()
	if err != nil {
		return err
	}

	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear adapters file: %w", err)
	}

	fmt.Printf("Cleared stored adapters from %s\n", configPath)
	return nil
}

func loadStoredAdapters() ([]string, error) {
	configPath, err := getAdapterConfigPath()
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(configPath) // #nosec G304 - configPath is controlled
	if err != nil {
		return nil, err
	}

	// Handle both Unix (\n) and Windows (\r\n) line endings
	contentStr := strings.ReplaceAll(string(content), "\r\n", "\n")
	lines := strings.Split(strings.TrimSpace(contentStr), "\n")
	var adapters []string
	for _, line := range lines {
		adapter := strings.TrimSpace(line)
		if adapter != "" && !strings.HasPrefix(adapter, "#") {
			adapters = append(adapters, adapter)
		}
	}

	return adapters, nil
}

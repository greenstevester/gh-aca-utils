# --- output(s) ---
Output docs/demo.gif
# Output docs/demo.mp4
# Output docs/frames/

# --- dependencies (fail fast if missing) ---
Require gh
Require bash
Require go

# --- look & feel ---
Set FontSize 32
Set Width 1200
Set Height 600
Set Padding 24
Set Margin 40
Set MarginFill "#0f172a"
Set WindowBar ColorfulRight
Set BorderRadius 16
Set TypingSpeed 40 ms
Set Theme "Catppuccin Frappe"

# --- build latest binary (hidden) ---
Hide
Type "clear"
Enter
Type "echo Building latest binary..."
Enter
Type "go build -o gh-aca-utils ."
Enter
Sleep 500ms
Type "clear"
Enter
Show

# --- title card ---
Type "# gh-aca-utils - GitHub CLI extension demo"
Enter
Sleep 800ms
Type "# Two commands: ip-port extraction + adapter toggling"
Enter
Sleep 800ms
Type "echo"
Enter

# --- show help first ---
Type "./gh-aca-utils --help"
Enter
Sleep 2s

# --- example 1: show ip-port command help ---
Type "echo"
Enter
Type "./gh-aca-utils ip-port --help"
Enter
Sleep 2s

# --- example 2: show flip-adapters command help ---
Type "echo"
Enter
Type "./gh-aca-utils flip-adapters --help"
Enter
Sleep 2s

# --- example 3: demonstrate with a real repo (if available) ---
Type "echo"
Enter
Type "# Example: scan a public repo for IP/port configurations"
Enter
Type "# ./gh-aca-utils ip-port --repo octocat/Hello-World --output table"
Enter
Sleep 1s

# --- example 4: demonstrate adapter flipping (dry-run) ---
Type "echo"
Enter
Type "# Example: toggle adapters in env/dev/parameters.properties"
Enter
Type "# ./gh-aca-utils flip-adapters --repo myorg/myrepo --env dev --adapters adapter1,adapter2 --dry-run"
Enter
Sleep 1s

# --- cleanup (hidden) ---
Hide
Type "rm -f gh-aca-utils"
Enter
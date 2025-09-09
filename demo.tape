# --- output(s) ---
Output docs/demo.gif
# Output docs/demo.mp4
# Output docs/frames/

# --- dependencies (fail fast if missing) ---
Require gh
Require bash
Require go

# --- look & feel ---
Set FontSize 18
Set Width 1200
Set Height 600
Set Padding 24
Set Margin 40
Set MarginFill "#0f172a"
Set WindowBar ColorfulRight
Set BorderRadius 16
Set TypingSpeed 40 ms
Set Theme "Catppuccin Frappe"

# --- title card ---
Type "# gh-aca-utils - An ACA GitHub CLI extension to help you work with github"
Enter
Sleep 800ms
Type "# Three main features: ip-port extraction + adapter management + adapter toggling"
Enter
Sleep 800ms
Type "echo"
Enter

# --- demo 1: Installation and help ---
Type "echo '=== DEMO 1: Installation and Command Overview ==='"
Enter
Type "# To install: gh extension install greenstevester/gh-aca-utils"
Enter
Sleep 2s
Type "echo"
Enter
Type "./gh-aca-utils --help"
Enter
Sleep 2s

# --- demo 2: IP/Port extraction ---
Type "echo"
Enter
Type "echo '=== DEMO 2: IP/Port Configuration Extraction ==='"
Enter
Type "./gh-aca-utils ip-port --help"
Enter
Sleep 2s
Type "echo"
Enter
Type "echo 'Scanning example repo for IP/port configurations...'"
Enter
Type "./gh-aca-utils ip-port --repo greenstevester/aca-example-repo --output table"
Enter
Sleep 3s

# --- demo 3: Adapter management ---
Type "echo"
Enter
Type "echo '=== DEMO 3: Adapter Management ==='"
Enter
Type "./gh-aca-utils set-adapters --help"
Enter
Sleep 2s
Type "echo"
Enter
Type "echo 'Example: Store adapters for reuse'"
Enter
Type "# ./gh-aca-utils set-adapters --adapters billing,search,payment"
Enter
Sleep 2s

# --- demo 4: Adapter flipping ---
Type "echo"
Enter
Type "echo '=== DEMO 4: Adapter Configuration Toggle ==='"
Enter
Type "./gh-aca-utils flip-adapters --help"
Enter
Sleep 2s
Type "echo"
Enter
Type "echo 'Current adapter states (dry-run):'"
Enter
Type "./gh-aca-utils flip-adapters --repo greenstevester/aca-example-repo --env dev --adapters billing,search --dry-run"
Enter
Sleep 3s

# --- demo 5: Flip adapters (simulated) ---
Type "echo"
Enter
Type "echo 'Flipping adapter states (simulated):'"
Enter
Type "./gh-aca-utils flip-adapters --repo greenstevester/aca-example-repo --env dev --adapters billing,search --dry-run=false"
Enter
Sleep 3s



# --- cleanup (hidden) ---
Hide
Type "rm -f gh-aca-utils"
Enter
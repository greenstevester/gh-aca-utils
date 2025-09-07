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
Type "# Two main features: ip-port extraction from the repo + adapter toggling"
Enter
Sleep 800ms
Type "echo"
Enter

# --- demo 1: Windows installation ---
Type "# Demo 1: gh-aca-utils extension - Installation on Windows"
Enter
Type "# Using Windows cmd Terminal or git bash:"
Enter
Type "gh extension install greenstevester/gh-aca-utils"
Enter
Sleep 2s
Type "echo"
Enter
Type "# Verify installation:"
Enter
Type "gh aca --help"
Enter
Sleep 2s
Type "echo"
Enter

# --- demo 2: show help ---
Type "# Demo 2: Command overview"
Enter
Type "gh aca --help"
Enter
Sleep 2s

# --- demo 3: show ip-port command help ---
Type "# Demo 3: IP/Port extraction command"
Enter
Type "gh aca ip-port --help"
Enter
Sleep 2s

# --- demo 4: show flip-adapters command help ---
Type "echo"
Enter
Type "# Demo 4: Adapter toggling command"
Enter
Type "gh aca flip-adapters --help"
Enter
Sleep 2s

# --- demo 5: show set-adapters command help ---
Type "echo"
Enter
Type "# Demo 5: Adapter management command"
Enter
Type "gh aca set-adapters --help"
Enter
Sleep 2s

# --- demo 6: demonstrate set-adapters usage ---
Type "echo"
Enter
Type "# Demo 6: Store frequently used adapters for reuse"
Enter
Type "# gh aca set-adapters --adapters billing,payment,notifications,search"
Enter
Sleep 1s
Type "# gh aca set-adapters --list"
Enter
Sleep 1s

# --- demo 7: demonstrate with a real repo (if available) ---
Type "echo"
Enter
Type "# Demo 7: Example usage - scan a repo for IP/port configurations"
Enter
Type "# gh aca ip-port --repo org/target-repo --output table"
Enter
Sleep 1s

# --- demo 8: demonstrate adapter flipping with stored adapters ---
Type "echo"
Enter
Type "# Demo 8: Example usage - toggle adapters using stored list"
Enter
Type "# gh aca flip-adapters --repo org/target-repo --env dev --dry-run"
Enter
Type "# (uses stored adapters from set-adapters command)"
Enter
Sleep 1s

# --- cleanup (hidden) ---
Hide
Type "rm -f gh-aca-utils"
Enter
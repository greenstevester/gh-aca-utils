# ⚡ gh-aca-utils — GitHub ACA CLI extension

gh extension to help with common tasks:

ip-port — clones a repo and extracts IP & Port key/value pairs, printing: IP Key, IP Value, Port Key, Port Value, File Path, Line Number — supports csv, table, and json output.

flip-adapters — toggles 0↔1 for specific adapter keys inside env/<ENV>/parameters.properties, with optional commit/branch/PR workflow — supports table and json output.

![demo](docs/demo.gif)

## Installation

1. Install the `gh` cli - see the [installation](https://github.com/cli/cli#installation)

   _Installation requires a minimum version (2.0.0) of the the GitHub CLI that supports extensions._

2. Install this extension:

   ```sh
   gh extension install greenstevester/gh-aca-utils
   ```

<details>
   <summary><strong>Manual Installation</strong></summary>

> If you want to install this extension manually, follow these steps:

1. clone the repo

   ```sh
   # git
   git clone https://github.com/greenstevester/gh-aca-utils

   # GitHub CLI
   gh repo clone greenstevester/gh-aca-utils
   ```

2. `cd` into it

   ```sh
   cd gh-aca-utils
   ```

3. add dependencies and build it

   ```sh
   go get && go build
   ```

4. install it locally
   ```sh
   gh extension install .
   ```
   </details>

## Usage

to run:

```sh
gh aca
```

to upgrade:

```sh
gh extension upgrade aca
```

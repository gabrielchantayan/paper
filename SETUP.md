---
contract: paper.setup/v1
product: paper
repository: https://github.com/gabrielchantayan/paper
installers:
  macos-linux: https://raw.githubusercontent.com/gabrielchantayan/paper/main/install.sh
  windows: https://raw.githubusercontent.com/gabrielchantayan/paper/main/install.ps1
agent_targets:
  claude-code: claude
  opencode: opencode
  kimi-code: kimi
  pi: pi
---

# Set Up Paper

Paper is a local-first work and friction log for coding agents. Set it up for the current user and only the agent environment executing these instructions.

## Procedure

1. Determine the operating system and the current agent identity. The supported agent identities and Paper targets are listed in the frontmatter. Do not guess from installed applications. If the current identity is unavailable or ambiguous, ask the user which target to use.

2. Install the latest checksum-verified Paper release.

   On macOS or Linux:

   ```sh
   script=$(mktemp)
   curl -fsSL https://raw.githubusercontent.com/gabrielchantayan/paper/main/install.sh -o "$script"
   sh "$script"
   rm "$script"
   ```

   On Windows PowerShell:

   ```powershell
   $script = Join-Path $env:TEMP "paper-install.ps1"
   Invoke-WebRequest https://raw.githubusercontent.com/gabrielchantayan/paper/main/install.ps1 -OutFile $script
   & $script
   Remove-Item $script
   ```

   If the release installer does not support the platform but Go 1.25 or newer is available, run:

   ```sh
   go install github.com/gabrielchantayan/paper/cmd/paper@latest
   ```

3. Ensure the installation directory is on `PATH`, then run `paper version`. Stop and report the error if Paper cannot execute.

4. Check whether `${XDG_CONFIG_HOME:-$HOME/.config}/paper/config.yaml` exists. If it does not exist, run `paper init`. If it exists, preserve it and report the configured vault instead of initializing again.

5. Run `paper agent doctor TARGET`, replacing `TARGET` with the current agent's mapped target.

6. Handle the reported status without modifying unrelated agent targets:

   - If the status is `installed and current`, make no change.
   - If it is `not installed`, run `paper agent install TARGET`.
   - If it is `outdated`, run `paper agent uninstall TARGET`, then `paper agent install TARGET`.
   - If it is `modified or unmanaged`, stop. Do not overwrite, rename, or delete the existing skill. Tell the user which path blocked setup.

7. Run `paper agent doctor TARGET` again. Setup succeeds only when it reports `installed and current` and the expected logging identity.

8. Report the Paper version, vault path, agent target, skill path, and final doctor status. Do not install skills for agents other than the current one unless the user explicitly asks.

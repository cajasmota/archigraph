# Manual Windows install

For locked-down, air-gapped, policy-restricted, or CMD-only machines where the
[PowerShell](install.md#windows--three-ways-to-install) or
[CMD](install.md#windows--three-ways-to-install) one-liners are not an option.
Every step below uses only Windows 10 1803+ built-ins and **requires no
administrator rights** — grafel installs entirely under your user profile.

> **TL;DR for the impatient:** download `grafel_<version>_windows_x86_64.zip`,
> extract it to `%USERPROFILE%\.grafel\bin`, add that folder to your user PATH,
> then run `grafel install`.

---

## 1. Pick the right release asset

Open the releases page and grab the **Windows x86_64** archive from the latest
release:

- https://github.com/cajasmota/grafel/releases/latest

Download the asset named:

```
grafel_<version>_windows_x86_64.zip
```

(for example `grafel_0.1.1_windows_x86_64.zip`), along with `checksums.txt` from
the same release if you want to verify the download (recommended).

> **ARM64 note:** there is no native `windows_arm64` build — the release links a
> C library (tree-sitter) and GitHub's runners have no Windows-ARM64
> cross-toolchain. Windows on ARM runs x64 binaries transparently via emulation,
> so use the `windows_x86_64` archive there too.

Save both files somewhere convenient, e.g. your `Downloads` folder. The commands
below assume:

```bat
cd %USERPROFILE%\Downloads
```

---

## 2. (Optional) Verify the checksum

In **CMD**, compute the SHA256 of the archive and compare it against the line for
your asset in `checksums.txt`:

```bat
certutil -hashfile grafel_0.1.1_windows_x86_64.zip SHA256
```

Then open `checksums.txt` and confirm the hash on the line ending in
`grafel_0.1.1_windows_x86_64.zip` matches (case-insensitive). If they differ, do
**not** proceed — re-download the asset.

---

## 3. Extract to the install folder

grafel lives under `%USERPROFILE%\.grafel\bin` (the same location the
PowerShell and CMD installers use). Create it and extract `grafel.exe` into it:

```bat
mkdir "%USERPROFILE%\.grafel\bin"
tar -xf grafel_0.1.1_windows_x86_64.zip -C "%USERPROFILE%\.grafel\bin"
```

`tar` is built into Windows 10 1803 and later. If you prefer, you can instead
right-click the `.zip` in File Explorer → **Extract All…**, then move the
extracted `grafel.exe` into `%USERPROFILE%\.grafel\bin`.

Confirm the binary is in place:

```bat
"%USERPROFILE%\.grafel\bin\grafel.exe" --version
```

---

## 4. Add the folder to your PATH

`grafel install` registers the MCP server, git hooks, and watchers — it does
**not** modify your OS `PATH`. So add the bin folder yourself. Pick either
method; both affect only your **user** PATH and need no admin rights.

### Option A — CMD (`setx`)

```bat
setx Path "%PATH%;%USERPROFILE%\.grafel\bin"
```

> `setx` writes the persisted user PATH but does **not** update the current
> window. Open a **new** terminal afterwards (or run
> `set "PATH=%PATH%;%USERPROFILE%\.grafel\bin"` to patch just this session).
>
> If your existing user PATH is very long (near the legacy 1024-char limit),
> prefer Option B so you don't risk truncation.

### Option B — System Properties UI

1. Press <kbd>Win</kbd> and type **"Edit environment variables for your
   account"**, open it.
2. Under **User variables for &lt;you&gt;**, select **Path** → **Edit…**.
3. Click **New** and paste the full path, e.g.
   `C:\Users\<you>\.grafel\bin`.
4. **OK** out of every dialog.
5. Open a **new** terminal so the change takes effect.

---

## 5. Register grafel and verify

In a **new** terminal (so PATH is picked up):

```bat
grafel install
```

This registers the MCP entry, installs git hooks and watchers, writes the IDE
rules files, and (for Claude Code) installs skills. It does not need admin
rights — the merged symlink fix lets a standard user install everything under
the user profile.

Then smoke-test:

```bat
grafel --version
grafel doctor
```

`grafel doctor` checks the install and detected AI coding tools. When it's
green, point grafel at your code:

```bat
grafel wizard
```

---

## Upgrading manually

Repeat steps 1–3 with the new release archive (overwrite `grafel.exe` in
`%USERPROFILE%\.grafel\bin`), then run `grafel install` again. PATH is already
set, so steps 4 is a one-time thing. Alternatively, once grafel is on PATH,
`grafel update` handles upgrades for you.

---

## Uninstalling

```bat
grafel uninstall
```

removes skills, MCP entries, and stops the daemon (graphs are preserved; add
`--purge` to remove them too). To finish, delete `%USERPROFILE%\.grafel` and
remove the bin folder from your user PATH (reverse of step 4).

---

See [install.md](install.md) for the full install matrix and
[troubleshooting.md](troubleshooting.md) if something doesn't line up.

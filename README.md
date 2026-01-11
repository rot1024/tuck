# 🛏️ tuck

> A lightweight terminal session manager — detach and reattach without the complexity.

**tuck** is like `tmux` or `screen`, but simpler. No window splitting, no status bars — just session management that stays out of your way.

## 🎯 Why I Built This

I wanted to run [Claude Code](https://github.com/anthropics/claude-code) remotely via SSH from my phone while on the go. But with `tmux`, `screen`, or `abduco`:

- 📱 **Screen rendering gets messy** — Alternate screen buffer doesn't play nice with mobile SSH clients
- 📜 **Can't scroll** — Terminal's native scrollback is hijacked
- 🤯 **Too much complexity** — I just want detach/attach, not window management

**tuck** solves this by *not* using the alternate screen buffer. Your terminal stays clean, scrollback works, and Claude Code renders perfectly.

## ✨ Features

- **📎 Session Management** — Create, attach, detach, and delete sessions
- **👥 Session Sharing** — Multiple clients can connect to the same session (pair programming!)
- **📜 Scrollback Works** — Unlike tmux/screen, your terminal's native scrollback keeps working
- **🎯 Zero Config** — No configuration files needed
- **🪶 Lightweight** — Single binary, minimal dependencies

## 📦 Installation

```bash
go install github.com/rot1024/tuck@latest
```

Or build from source:

```bash
git clone https://github.com/rot1024/tuck.git
cd tuck
go build
```

## 🚀 Quick Start

```bash
# Start a new session (auto-generated name from current directory)
tuck
tuck new

# Start with a specific command (auto-generated name)
tuck new bash

# Start with a specific name
tuck create myproject

# Start with a specific name and command
tuck create myproject bash

# List sessions
tuck list

# Attach to an existing session
tuck attach myproject

# Delete a session
tuck delete myproject
```

## ⌨️ Keybindings

| Key | Action |
|-----|--------|
| `~.` | Detach from session (default, like SSH) |
| `~~` | Send literal `~` character |

### SSH-Style Escape Sequence

Like SSH, you can detach by pressing `~.` (tilde then period) after a newline. This works great with Claude Code and other applications that capture control keys.

### Custom Detach Key

You can configure detach keys via flags or environment variables:

```bash
# Single key via flag
tuck -d '~.' new
tuck -d ctrl-a attach mysession

# Multiple keys via flags
tuck -d '`.' -d ctrl-a new

# Via environment variables
export TUCK_DETACH_KEY='`.'
export TUCK_DETACH_KEY_1='~.'
export TUCK_DETACH_KEY_2=ctrl-a
tuck new
```

Supported formats:
- Escape sequences: `` `. ``, `~.` (character + period, triggered after Enter)
- Control keys: `ctrl-a`, `ctrl-]`, `^a`, `^A`

## 💬 Messages

tuck shows helpful status messages:

```
[tuck: ✨ created "myproject" (~. to detach)]
[tuck: 🔗 attached "myproject" (~. to detach)]
[tuck: 👋 detached "myproject"]
[tuck: 🏁 ended "myproject"]
```

Use `--quiet` or `-q` to suppress messages.

## 🤔 Why tuck?

| Feature | tmux | screen | abduco | tuck |
|---------|------|--------|--------|------|
| Session detach/attach | ✅ | ✅ | ✅ | ✅ |
| Multiple clients | ✅ | ✅ | ✅ | ✅ |
| Window splitting | ✅ | ✅ | ❌ | ❌ |
| Status bar | ✅ | ✅ | ❌ | ❌ |
| Native scrollback | ❌ | ❌ | ❌ | ✅ |
| Zero config | ❌ | ❌ | ✅ | ✅ |

**tuck** doesn't use the alternate screen buffer, so your terminal's scrollback buffer remains functional. Perfect for tools like Claude Code that rely on terminal rendering.

## 📝 Commands

```
tuck                      # Create and attach to a new session (auto-named)
tuck new [cmd]            # Create a new session with auto-generated name
tuck create <name> [cmd]  # Create a new session with specified name
tuck attach <name>        # Attach to an existing session
tuck list                 # List all sessions
tuck delete <name>        # Delete a session
```

### Aliases

- `tuck n` → `tuck new`
- `tuck c` → `tuck create`
- `tuck a` → `tuck attach`
- `tuck ls` → `tuck list`
- `tuck rm` → `tuck delete`

## 📄 License

MIT

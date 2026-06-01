# clip98

![gif](https://media.giphy.com/media/2SFebJNbVQCHI1PoS2/giphy.gif)

**Learn more about this in my latest blog post: https://giuliozausa.dev/posts/bm-1-clipboard/**

Bidirectional clipboard sync between any Windows machine (from 95 to 10) and any other machine via serial port. Two host-side implementations are provided: a standalone **Go binary** and a **Node.js** script.

## How It Works

The two sides communicate over a **serial port** (9600 baud, 8N1, no parity). Each side monitors its local clipboard for changes and pushes updates to the other:

- **Windows → Host**: clipboard text terminated with a **form feed** (`\f`) delimiter. The host side buffers data until it receives `\f`, then sets the clipboard.
- **Host → Windows**: raw clipboard text with no delimiter. The Windows side reads until a serial read timeout, then sets the clipboard.

## Quick Start

### 1. Build the Windows executable

Using the [VC6 Docker image](https://github.com/giulioz/vc6-docker):

```bash
docker run --rm -v $(pwd):/prj giulioz/vc6-docker \
  wine /opt/vc/BIN/CL.EXE z:\\prj\\cpp\\clip98.cpp \
  /GX /IZ:\\opt\\vc\\INCLUDE \
  /link /LIBPATH:Z:\\opt\\vc\\LIB user32.lib \
  /out:Z:\\prj\\clip98.exe
```

Or download the latest CI-built binary from [Releases](https://github.com/giulioz/clip98/releases).

### 2. Run the host side

**Option A: Go binary**

Single static binary — **no runtime, no dependencies, no installation**. Download from [Releases](https://github.com/giulioz/clip98/releases):

```bash
# Download for macOS Apple Silicon
curl -LO https://github.com/giulioz/clip98/releases/download/latest/clip98-darwin-arm64
chmod +x clip98-darwin-arm64
./clip98-darwin-arm64
```

To build from source (requires [Go](https://go.dev/dl/)):

```bash
cd go
go build -o clip98 .
./clip98
```

**Option B: Node.js**

```bash
cd js
npm install
npm start
```

Both implementations **auto-detect** the QEMU serial port. If auto-detection picks the wrong device, override it:

```bash
# Go (rename the downloaded binary, or use the full name)
./clip98-darwin-arm64 /dev/ttys019
SERIAL_PORT=/dev/ttys019 ./clip98-darwin-arm64

# Node.js
npm start -- /dev/ttys019
SERIAL_PORT=/dev/ttys019 npm start
```

### 3. QEMU Setup

Add `-serial pty` to your QEMU command line. QEMU will assign a PTY at startup — the host script will detect it automatically.

## Serial Protocol

| Parameter | Value |
|-----------|-------|
| Baud rate | 9600 |
| Data bits | 8 |
| Stop bits | 1 |
| Parity | None |
| Encoding | Raw bytes (ANSI on Win98, host encoding on host side) |
| Delimiter | Form feed (`0x0C` / `\f`), Windows → Host only |

## Downloads

Pre-built binaries for all platforms are available on the [Releases page](https://github.com/giulioz/clip98/releases):

| File | Platform |
|------|----------|
| `clip98.exe` | Windows 95/98/ME/.../10 (guest) |
| `clip98-darwin-arm64` | macOS Apple Silicon (host) |
| `clip98-darwin-amd64` | macOS Intel (host) |
| `clip98-linux-amd64` | Linux x86_64 (host) |
| `clip98-windows-amd64.exe` | Windows x86_64 (host) |

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `Error opening serial port` (Windows) | The COM port may be in use. Check `mode` in Command Prompt. |
| `Permission denied` (macOS/Linux) | Run `sudo chmod 666 /dev/ttysXXX` or add your user to the `dialout` group. |
| Clipboard not syncing | Check that both sides are running and the serial port is connected. |
| Build warnings about C4530 | Ensure you pass `/GX` to the compiler (included in the command above). |
| Garbled non-Latin characters | Win98 uses ANSI code pages, not Unicode. Characters outside the system code page will be corrupted. |

## License

MIT

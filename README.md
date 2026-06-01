# clip98

![gif](https://media.giphy.com/media/2SFebJNbVQCHI1PoS2/giphy.gif)

**Learn more about this in my latest blog post: https://giuliozausa.dev/posts/bm-1-clipboard/**

Bidirectional clipboard sync between any Windows machine (from 95 to 10) and any other Node.js–capable machine via serial port.

## How It Works

The two sides communicate over a **serial port** (9600 baud, 8N1, no parity). Each side monitors its local clipboard for changes and pushes updates to the other:

- **Windows → Node**: clipboard text terminated with a **form feed** (`\f`) delimiter. The Node.js side uses a delimiter parser to split messages.
- **Node → Windows**: raw clipboard text with no delimiter. The Windows side reads until a serial read timeout, then sets the clipboard.

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

### 2. Run the Node.js side

```bash
cd js
npm install
npm start
```

The serial port is **auto-detected** — it scans `/dev/ttys*` and picks the most recently opened PTY (which is almost always QEMU's). No manual configuration needed in most cases.

If auto-detection picks the wrong device, you can override it:

```bash
# Via CLI argument (note the -- so npm passes the arg to the script)
npm start -- /dev/ttys019

# Or via environment variable
SERIAL_PORT=/dev/ttys019 npm start
```

### 3. QEMU Setup

Add `-serial pty` to your QEMU command line. QEMU will assign a PTY at startup — the Node.js script will detect it automatically.

## Serial Protocol

| Parameter | Value |
|-----------|-------|
| Baud rate | 9600 |
| Data bits | 8 |
| Stop bits | 1 |
| Parity | None |
| Encoding | Latin-1 (raw bytes) |
| Delimiter | Form feed (`0x0C` / `\f`) |

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `Error opening serial port` (Windows) | The COM port may be in use. Check `mode` in Command Prompt. |
| `Error: Permission denied` (macOS/Linux) | Run `sudo chmod 666 /dev/ttysXXX` or add your user to the `dialout` group. |
| `Serial port error: ...` (Node.js) | Verify the port path matches the one QEMU assigned. |
| Clipboard not syncing | Check that both sides are running and the serial cable/port is connected. |
| Build warnings about C4530 | Ensure you pass `/GX` to the compiler (included in the command above). |

## Downloads

- **Latest release**: [Releases page](https://github.com/giulioz/clip98/releases)
- **Pinned `latest` tag**: [clip98.exe](https://github.com/giulioz/clip98/releases/download/latest/clip98.exe) (development build from `main`)

## License

MIT

const { execSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const ncp = require("copy-paste");
const { SerialPort } = require("serialport");
const { DelimiterParser } = require("@serialport/parser-delimiter");

// --- Configuration ---
const POLL_INTERVAL_MS = 1000;
const ENCODING = "latin1"; // Matches the raw byte handling on the Win95 C++ side

// --- Detect own terminal so we never connect to it ---
const ownTty = getOwnTty();

// --- Auto-detect or use explicit port ---
const PORT_PATH = process.env.SERIAL_PORT || process.argv[2] || autoDetectPty();

if (!PORT_PATH) {
  console.error(
    "No serial port found.\n" +
      "  Auto-detection looks for a QEMU process holding a PTY device.\n" +
      "  Override with SERIAL_PORT env var or CLI argument.\n" +
      "\n" +
      "  Example: npm start -- /dev/ttys019\n" +
      "       or: SERIAL_PORT=/dev/ttys019 npm start\n" +
      "       or: npm start  (auto-detect)"
  );
  process.exit(1);
}

if (PORT_PATH === ownTty) {
  console.error(
    `Refusing to connect to own terminal (${ownTty}).\n` +
      "Pass the QEMU serial port explicitly:\n" +
      "  npm start -- /dev/ttys0XX"
  );
  process.exit(1);
}

console.log(`Using serial port: ${PORT_PATH}`);

// --- State ---
let lastClipboard = null;
let sequence = 0; // monotonic log counter for tracing message flow

// --- Serial Port Setup ---
const port = new SerialPort({ path: PORT_PATH, baudRate: 9600 });

port.on("open", () => {
  console.log(`Connected to ${PORT_PATH}`);
});

port.on("error", (err) => {
  console.error(`Serial port error: ${err.message}`);
  process.exit(1);
});

port.on("close", () => {
  if (!running) return; // Expected close from shutdown()
  console.error("Serial port closed unexpectedly.");
  process.exit(1);
});

// --- Incoming data → local clipboard ---
const parser = port.pipe(new DelimiterParser({ delimiter: "\f" }));

parser.on("data", (data) => {
  try {
    const text = data.toString(ENCODING);
    if (text.length === 0) return;

    lastClipboard = text;
    sequence++;
    console.log(`[recv #${sequence}] ${text}`);
    ncp.copy(text);
  } catch (err) {
    console.error("Error processing incoming data:", err.message);
  }
});

// --- Local clipboard → outgoing data ---
let running = true;

function pollClipboard() {
  if (!running) return;
  try {
    const data = ncp.paste();
    if (data !== lastClipboard) {
      sequence++;
      console.log(`[send #${sequence}] ${data}`);
      port.write(Buffer.from(data, ENCODING));
      // Note: no \f delimiter sent — the Windows ReceiveData reads until
      // serial timeout, so \f would end up on the guest clipboard.
      lastClipboard = data;
    }
  } catch (err) {
    // Silently ignore clipboard read failures (e.g. empty or unsupported format)
  }
  setTimeout(pollClipboard, POLL_INTERVAL_MS);
}

pollClipboard();

// --- Graceful shutdown ---
let shuttingDown = false;
function shutdown() {
  if (shuttingDown) return;
  shuttingDown = true;
  running = false;
  console.log("\nShutting down...");
  port.close(() => process.exit(0));
  // Fallback: force exit after 2s if close callback never fires
  setTimeout(() => process.exit(0), 2000).unref();
}

process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);

// Also catch Ctrl+C via raw stdin as a fallback —
// when connected to a wrong PTY, the serial data flood
// can prevent SIGINT from being delivered.
if (process.stdin.isTTY) {
  process.stdin.setRawMode(true);
  process.stdin.on("data", (key) => {
    // Ctrl+C is byte 0x03
    if (key[0] === 0x03) shutdown();
  });
}

// --- Helpers ---

/** Get this script's own terminal device path. */
function getOwnTty() {
  // Try the `tty` command (most reliable on macOS)
  try {
    const tty = execSync("tty 2>/dev/null", { encoding: "utf-8" }).trim();
    if (tty.startsWith("/dev/")) return tty;
  } catch {}
  // Fallback: readlink /dev/fd/0
  try {
    const tty = fs.readlinkSync("/dev/fd/0").trim();
    if (tty.startsWith("/dev/")) return tty;
  } catch {}
  return null;
}

/**
 * Auto-detect the QEMU serial port PTY.
 *
 * macOS:  QEMU holds /dev/ptmx (PTY master). The lsof DEVICE column
 *         shows "major,minor" — the minor number maps to /dev/ttysXXX.
 * Linux:  QEMU holds /dev/ptmx. lsof shows the slave as /dev/pts/N directly.
 */
function autoDetectPty() {
  const exclude = ownTty ? new Set([ownTty]) : new Set();
  const isMac = process.platform === "darwin";

  // Find QEMU PIDs
  let qemuPids = null;
  for (const cmd of ["pgrep -f qemu-system", "pgrep qemu"]) {
    try {
      qemuPids = execSync(cmd, { encoding: "utf-8" }).trim().replace(/\n/g, ",");
      if (qemuPids) break;
    } catch {}
  }

  if (qemuPids) {
    try {
      const output = execSync(`lsof -p ${qemuPids} 2>/dev/null`, {
        encoding: "utf-8",
        timeout: 5000,
      });

      // macOS: resolve /dev/ptmx minor number to /dev/ttysXXX slave
      if (isMac) {
        for (const line of output.split("\n")) {
          const cols = line.split(/\s+/);
          if (cols.length < 9) continue;

          const name = cols[cols.length - 1];
          if (name !== "/dev/ptmx") continue;

          // lsof columns: COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME
          //                0      1   2    3  4    5      6         7    8
          const deviceCol = cols[5]; // e.g. "15,17"
          const minor = parseInt(deviceCol.split(",")[1], 10);
          if (isNaN(minor)) continue;

          const slavePath = `/dev/ttys${String(minor).padStart(3, "0")}`;
          if (fs.existsSync(slavePath) && !exclude.has(slavePath)) {
            console.log(`Auto-detected QEMU serial port: ${slavePath}`);
            return slavePath;
          }
        }
      }

      // Linux: find /dev/pts/N entries (not fd 0-2)
      for (const line of output.split("\n")) {
        const cols = line.split(/\s+/);
        if (cols.length < 9) continue;

        const fd = cols[3];
        const name = cols[cols.length - 1];
        if (/^[012]/.test(fd)) continue;
        if (!name || !name.startsWith("/dev/pts/")) continue;
        if (exclude.has(name)) continue;

        console.log(`Auto-detected QEMU serial port: ${name}`);
        return name;
      }
    } catch {}
  }

  // Fallback: newest PTY device (excluding own terminal)
  try {
    const ptyDir = isMac ? "/dev" : "/dev/pts";
    const ptyPattern = isMac ? /^ttys\d+$/ : /^\d+$/;
    const prefix = isMac ? "/dev/" : "/dev/pts/";

    if (!fs.existsSync(ptyDir)) return null;

    const candidates = fs
      .readdirSync(ptyDir)
      .filter((name) => ptyPattern.test(name))
      .map((name) => {
        const fullPath = `${prefix}${name}`;
        if (exclude.has(fullPath)) return null;
        try {
          const stat = fs.statSync(fullPath);
          return { path: fullPath, mtime: stat.mtime.getTime() };
        } catch {
          return null;
        }
      })
      .filter(Boolean)
      .sort((a, b) => b.mtime - a.mtime);

    if (candidates.length === 0) return null;

    console.log(
      `No QEMU process found via lsof, falling back to newest PTY: ${candidates[0].path}`
    );
    return candidates[0].path;
  } catch {
    return null;
  }
}

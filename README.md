# DonkeyShare Core (`DonkeyShare-Core`)

The core file-handling, transfer operations, and server engine of **DonkeyShare** written in Go.

This repository houses the core operational logic of the file-sharing application, completely decoupled from any specific GUI layout, desktop bridge, or Web UI frontend.

---

## Technical Overview

The engine is structured into two main packages:

### 1. `config`
Handles parsing, host lookup, and verification constraints:
* **Dynamic IP Detection:** Resolves active network IP connections on the host machine.
* **TCP Port Selection:** Auto-scans and verifies available TCP ports in a given range.
* **Upload Limits:** Parses human-readable file constraints (e.g., `100M`, `10G`) into system bytes.
* **Shared Paths:** Configures directories designated for host/client handshakes.

### 2. `server`
Hosts the direct transfer and static HTTP/HTTPS routers:
* **High-Speed Transfer handlers:** Processes multi-threaded file uploads, chunked downloads, and directory zip generation.
* **Secure Connections (SSL):** Supports localized HTTPS servers using generated keys.
* **Client Handshakes:** Handles PIN validation, session tokens, and connection timeouts.
* **Log Capturing:** Pipes diagnostic server logging details to binding subscribers.

---

## ⚠️ Compilation and UI Requirement

> [!IMPORTANT]
> **This repository is NOT a standalone GUI application.** 
> It houses only the low-level Go core operations and file-handling logic. 

If you want to compile and run a complete client application:
1. **Develop a UI Frontend:** You must build your own web assets (React, Vue, or Vanilla HTML/JS) to serve as the user interface.
2. **Implement a GUI Bridge:** You will need to write a bridge package (such as webview or wails) to mount your UI, bind the core APIs, and initialize native operating system dialogs (such as directory pickers).

---

## Building and Testing

To verify the core files compile and behave as expected, you can run the Go test suites:

```bash
# Run tests for all packages
go test -v ./...
```

---

## License

### Proprietary (View-Only License)

**Copyright (c) 2026. All rights reserved.**

This source code is made available solely for review, inspection, and educational purposes. 

* **Permission:** You are allowed to inspect and read this source code.
* **Prohibitions:** You may **not** copy, modify, distribute, publish, sublicense, merge, package, or use this software (or any portion thereof) for any commercial or non-commercial purpose whatsoever.

For inquiries regarding licensing or alternative arrangements, please contact the repository owner.

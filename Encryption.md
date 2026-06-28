# DonkeyShare Local TLS Encryption (Secure Sharing)

This document explains how the **Secure Sharing** option works in DonkeyShare, why your web browser shows certificate warnings, and how to safely navigate them.

---

## What is Secure Sharing?

When you toggle **Secure Sharing** on, DonkeyShare runs its file-sharing server over **HTTPS** (Hypertext Transfer Protocol Secure) instead of standard HTTP. 

1. **End-to-End Encryption:** All traffic, file transfers, and credentials (like the Access PIN) exchanged between the receiver and sender are encrypted using TLS (Transport Layer Security).
2. **Local Security:** This prevents anyone on your local Wi-Fi network from sniffing the network packets or intercepting files during transit.

---

## The "Your connection is not private" Warning

When a receiver connects to a secure DonkeyShare link (e.g., `https://192.168.0.195:9000`), modern browsers (Chrome, Edge, Firefox, Safari) will display a security warning (such as `NET::ERR_CERT_AUTHORITY_INVALID`).

### Why does this warning appear?

* **Self-Signed Certificate:** DonkeyShare dynamically generates a self-signed TLS/SSL certificate directly on your host machine to encrypt the connection.
* **Lack of CA Signature:** Public Certificate Authorities (like Let's Encrypt or DigiCert) only issue certificates to registered, public domain names (e.g., `google.com`). They **cannot** sign certificates for local, private network IP addresses (like `192.168.x.x` or `127.0.0.1`).
* **Browser Trust Rules:** Because the browser cannot verify the certificate creator against a trusted public root authority, it displays a safety warning.

---

## How to Proceed Safely

Since this server is running entirely inside your local area network (LAN) on your own device, it is **100% safe** to bypass this warning:

### 1. Google Chrome & Microsoft Edge:
1. On the warning screen, click **Advanced** at the bottom left.
2. Click **Proceed to [IP Address] (unsafe)**.

### 2. Mozilla Firefox:
1. Click **Advanced...** on the error page.
2. Click **Accept the Risk and Continue**.

### 3. Safari (macOS/iOS):
1. Click **Show Details** on the warning banner.
2. Click **visit this website** at the bottom.
3. Confirm by selecting **Visit Website** and authenticate with your passcode/TouchID.

---

## Technical Details

DonkeyShare generates key pairs using standard crypto packages in Go:
* **Algorithm:** RSA 2048-bit or ECDSA (P-256).
* **Scope:** The certificate is configured with Subject Alternative Names (SANs) matching the sender's active local interface IP addresses.

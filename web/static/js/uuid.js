/*
 * UUIDv7 generator (RFC 9562).
 *
 * Format (hex, lowercase):
 *   xxxxxxxx-xxxx-7xxx-yxxx-xxxxxxxxxxxx
 *   - 48 bit unix timestamp (ms) di bagian high
 *   - 4 bit version = 7
 *   - 2 bit variant = 10
 *   - sisanya random
 *
 * window.uuidv7() -> string
 */
(function () {
  "use strict";

  function toHex(byte) {
    return byte.toString(16).padStart(2, "0");
  }

  function uuidv7() {
    const ts = Date.now(); // ms unix
    // 48 bit timestamp -> 6 bytes.
    const tsBytes = new Uint8Array(6);
    // JS Number aman untuk integer sampai 2^53; 48 bit OK.
    let t = ts;
    for (let i = 5; i >= 0; i--) {
      tsBytes[i] = t & 0xff;
      t = Math.floor(t / 256);
    }

    const rand = new Uint8Array(10);
    if (window.crypto && typeof window.crypto.getRandomValues === "function") {
      window.crypto.getRandomValues(rand);
    } else {
      for (let i = 0; i < rand.length; i++) rand[i] = Math.floor(Math.random() * 256);
    }

    // Compose 16 bytes: [ts(6)] + [rand(10)]
    const bytes = new Uint8Array(16);
    bytes.set(tsBytes, 0);
    bytes.set(rand, 6);

    // Version: byte 6 bits 4-7 = 0111 (v7)
    bytes[6] = (bytes[6] & 0x0f) | 0x70;
    // Variant: byte 8 bits 6-7 = 10
    bytes[8] = (bytes[8] & 0x3f) | 0x80;

    const hex = Array.from(bytes, toHex).join("");
    return (
      hex.slice(0, 8) +
      "-" +
      hex.slice(8, 12) +
      "-" +
      hex.slice(12, 16) +
      "-" +
      hex.slice(16, 20) +
      "-" +
      hex.slice(20, 32)
    );
  }

  window.uuidv7 = uuidv7;
})();

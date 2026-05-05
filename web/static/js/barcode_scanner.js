/*
 * Tokobangunan barcode scanner helper.
 *
 * Mendeteksi handheld USB barcode scanner (HID keyboard emulation) yang
 * mengirim karakter cepat lalu Enter. Dibedakan dari typing manual via
 * timing antar karakter (scanner > 30 char/detik).
 *
 * Public API:
 *   window.barcodeScanner(onScan)
 *     - onScan(code: string): callback ketika scan terdeteksi.
 *     - Skip kalau fokus berada di input/textarea/select/contenteditable.
 *
 * Catatan:
 *   - keypress dipakai (bukan keydown) supaya cuma karakter printable yang masuk
 *     buffer; Enter tetap terdeteksi.
 *   - Buffer di-reset jika gap antar key > 200ms (artinya bukan scanner).
 */
(function () {
  "use strict";

  window.barcodeScanner = function (onScan, opts) {
    const o = opts || {};
    const minLength = o.minLength || 4;
    const idleResetMs = o.idleResetMs || 200;

    let buffer = "";
    let lastTime = 0;

    function isEditable(el) {
      if (!el) return false;
      const tag = (el.tagName || "").toLowerCase();
      if (tag === "input" || tag === "textarea" || tag === "select") return true;
      if (el.isContentEditable) return true;
      return false;
    }

    document.addEventListener("keypress", function (e) {
      // Abaikan kalau user sedang ngetik di field.
      if (isEditable(e.target)) return;

      const now = Date.now();
      if (now - lastTime > idleResetMs) {
        buffer = "";
      }
      lastTime = now;

      if (e.key === "Enter") {
        if (buffer.length >= minLength) {
          try { onScan(buffer); } catch (err) { /* noop */ }
        }
        buffer = "";
        return;
      }
      if (e.key && e.key.length === 1) {
        buffer += e.key;
      }
    });
  };
})();

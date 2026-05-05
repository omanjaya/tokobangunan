/*
 * Penjualan offline behavior injector.
 *
 * Tugas:
 *   1. Inject hidden input client_uuid (UUIDv7) ke form penjualan saat mount.
 *   2. Tandai form data-offline-able="true".
 *   3. Auto-save draft ke localStorage saat field berubah, restore saat reload.
 *   4. Intercept submit: kalau offline, enqueue ke window.tbSync dan tampil toast.
 *
 * Selector form: prioritas #penjualan-form, fallback form[data-form="penjualan"],
 * fallback form yang action-nya /penjualan dan method POST.
 */
(function () {
  "use strict";

  const DRAFT_KEY = "tb:penjualan:draft";

  function findForm() {
    let form = document.getElementById("penjualan-form");
    if (form) return form;
    form = document.querySelector('form[data-form="penjualan"]');
    if (form) return form;
    const candidates = document.querySelectorAll('form[method="post" i], form');
    for (const f of candidates) {
      const action = (f.getAttribute("action") || "").trim();
      if (action === "/penjualan" || action.endsWith("/penjualan")) return f;
    }
    return null;
  }

  function ensureClientUUID(form) {
    let input = form.querySelector('input[name="client_uuid"]');
    if (!input) {
      input = document.createElement("input");
      input.type = "hidden";
      input.name = "client_uuid";
      form.appendChild(input);
    }
    if (!input.value) {
      const fn = window.uuidv7;
      if (typeof fn === "function") {
        input.value = fn();
      }
    }
    return input;
  }

  function markOfflineAble(form) {
    if (!form.hasAttribute("data-offline-able")) {
      form.setAttribute("data-offline-able", "true");
    }
  }

  // ---------- Draft autosave ----------------------------------------------

  function serializeForm(form) {
    const data = {};
    const fd = new FormData(form);
    for (const [k, v] of fd.entries()) {
      if (typeof v !== "string") continue;
      // Multi-value: simpan sebagai array.
      if (data[k] !== undefined) {
        if (!Array.isArray(data[k])) data[k] = [data[k]];
        data[k].push(v);
      } else {
        data[k] = v;
      }
    }
    return data;
  }

  function restoreDraft(form) {
    let raw;
    try {
      raw = localStorage.getItem(DRAFT_KEY);
    } catch (e) {
      return;
    }
    if (!raw) return;
    let data;
    try {
      data = JSON.parse(raw);
    } catch (e) {
      return;
    }
    if (!data || typeof data !== "object") return;
    // Hanya restore field yang masih kosong (non-destructive).
    Object.keys(data).forEach((name) => {
      if (name === "client_uuid") return; // jangan timpa, sudah baru.
      const val = data[name];
      const fields = form.querySelectorAll('[name="' + cssEscape(name) + '"]');
      if (!fields.length) return;
      fields.forEach((f, idx) => {
        if (f.type === "hidden" || f.type === "submit" || f.type === "button") return;
        if (f.type === "checkbox" || f.type === "radio") {
          const target = Array.isArray(val) ? val : [val];
          if (target.indexOf(f.value) >= 0) f.checked = true;
        } else if (!f.value) {
          if (Array.isArray(val)) {
            if (val[idx] !== undefined) f.value = val[idx];
          } else {
            f.value = val;
          }
        }
      });
    });
  }

  function cssEscape(s) {
    if (window.CSS && typeof window.CSS.escape === "function") return window.CSS.escape(s);
    return s.replace(/(["\\\[\]])/g, "\\$1");
  }

  function saveDraft(form) {
    try {
      const data = serializeForm(form);
      localStorage.setItem(DRAFT_KEY, JSON.stringify(data));
    } catch (e) {
      /* quota / private mode -> ignore */
    }
  }

  function clearDraft() {
    try {
      localStorage.removeItem(DRAFT_KEY);
    } catch (e) {
      /* ignore */
    }
  }

  // ---------- Submit interception ----------------------------------------

  async function offlineSubmit(form, e) {
    if (navigator.onLine) return; // online -> biarkan native submit.
    e.preventDefault();
    e.stopPropagation();

    const fd = new FormData(form);
    const params = new URLSearchParams();
    for (const [k, v] of fd.entries()) {
      if (typeof v === "string") params.append(k, v);
    }
    const body = params.toString();
    const action = form.getAttribute("action") || "/penjualan";
    const method = (form.getAttribute("method") || "POST").toUpperCase();
    const clientUUID = fd.get("client_uuid") || "";

    if (!window.tbSync) {
      if (window.app) window.app.toast("error", "Offline", "Modul sync belum siap");
      return;
    }

    try {
      await window.tbSync.enqueue({
        url: new URL(action, window.location.origin).toString(),
        method,
        headers: { "Content-Type": "application/x-www-form-urlencoded; charset=UTF-8" },
        body,
        contentType: "application/x-www-form-urlencoded; charset=UTF-8",
        client_uuid: String(clientUUID || ""),
      });

      if (window.app) {
        window.app.toast(
          "warning",
          "Tersimpan offline",
          "Transaksi akan dikirim otomatis saat koneksi kembali"
        );
      }
      clearDraft();
      // Generate uuid baru biar form siap untuk transaksi berikutnya.
      const uuidInput = form.querySelector('input[name="client_uuid"]');
      if (uuidInput && typeof window.uuidv7 === "function") uuidInput.value = window.uuidv7();
      // Reset form.
      try {
        form.reset();
      } catch (er) {
        /* ignore */
      }
    } catch (err) {
      if (window.app) {
        window.app.toast("error", "Gagal antri offline", String(err && err.message ? err.message : err));
      }
    }
  }

  // ---------- Init -------------------------------------------------------

  function init() {
    const form = findForm();
    if (!form) return;
    markOfflineAble(form);
    ensureClientUUID(form);
    restoreDraft(form);

    let saveTimer = null;
    form.addEventListener("input", () => {
      clearTimeout(saveTimer);
      saveTimer = setTimeout(() => saveDraft(form), 400);
    });
    form.addEventListener("change", () => saveDraft(form));

    form.addEventListener("submit", (e) => {
      // Kalau online, hapus draft saat submit (best effort).
      if (navigator.onLine) {
        // Biarkan submit normal; clear draft setelah delay agar tidak ganggu.
        setTimeout(clearDraft, 500);
        return;
      }
      offlineSubmit(form, e);
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();

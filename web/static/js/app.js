/*
 * Tokobangunan client helpers.
 * No framework dependency selain Alpine + HTMX yang sudah di-load.
 *
 * Public API (window.app):
 *   app.toast(variant, title, message?, durationMs?)
 *   app.formatThousand(value)        - format angka pakai pemisah ribuan
 *   app.parseThousand(value)         - balikan dari formatThousand
 *   app.confirm(message, onConfirm)  - konfirmasi sederhana (window.confirm fallback)
 */
(function () {
  "use strict";

  const app = {};

  // ---------- CSRF (double-submit cookie) ----------------------------------
  // Server-side CSRF middleware (echo CSRF) sets the token on cookie "_csrf"
  // and validates form field "csrf_token" or header "X-CSRF-Token" against
  // that cookie value. We expose the token to:
  //   1. htmx requests (auto-add X-CSRF-Token header)
  //   2. Plain HTML form POSTs (auto-inject hidden csrf_token before submit)
  // This means individual templates do NOT need to render the hidden input
  // themselves — it's added at submit-time by this script.
  app.getCsrfToken = function () {
    const m = document.cookie.match(/(?:^|;\s*)_csrf=([^;]+)/);
    return m ? decodeURIComponent(m[1]) : "";
  };

  // htmx: inject header on every htmx-issued request (POST/PUT/DELETE etc.)
  document.addEventListener("htmx:configRequest", function (e) {
    const t = app.getCsrfToken();
    if (t) {
      e.detail.headers["X-CSRF-Token"] = t;
    }
  });

  // Plain form POST/PUT/DELETE: inject hidden csrf_token if missing.
  // Capture-phase listener so we run before any other submit handler.
  document.addEventListener(
    "submit",
    function (e) {
      const form = e.target;
      if (!form || form.tagName !== "FORM") return;
      const method = (form.getAttribute("method") || "get").toLowerCase();
      if (method === "get") return;
      // Skip if template already provides the token.
      if (form.querySelector('input[name="csrf_token"]')) return;
      const t = app.getCsrfToken();
      if (!t) return;
      const input = document.createElement("input");
      input.type = "hidden";
      input.name = "csrf_token";
      input.value = t;
      form.appendChild(input);
    },
    true
  );

  // ---------- Toast ---------------------------------------------------------

  app.toast = function (variant, title, message, durationMs) {
    const detail = {
      variant: variant || "info",
      title: title || "",
      message: message || "",
      duration: typeof durationMs === "number" ? durationMs : 5000,
    };
    window.dispatchEvent(new CustomEvent("toast:add", { detail: detail }));
  };

  // HTMX trigger event "showToast" -> push ke toast container.
  document.addEventListener("showToast", function (event) {
    const data = event.detail || {};
    app.toast(data.variant, data.title, data.message, data.duration);
  });

  // ---------- Number formatting --------------------------------------------

  app.formatThousand = function (value) {
    if (value === null || value === undefined || value === "") return "";
    const digits = String(value).replace(/[^\d-]/g, "");
    if (digits === "" || digits === "-") return digits;
    const negative = digits.startsWith("-");
    const num = digits.replace(/-/g, "");
    return (negative ? "-" : "") + num.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  };

  app.parseThousand = function (value) {
    if (value === null || value === undefined) return 0;
    const cleaned = String(value).replace(/[^\d-]/g, "");
    if (cleaned === "" || cleaned === "-") return 0;
    return parseInt(cleaned, 10) || 0;
  };

  // Auto-bind <input data-mask="thousand"> untuk format ribuan.
  document.addEventListener("input", function (event) {
    const el = event.target;
    if (!(el instanceof HTMLInputElement)) return;
    if (el.dataset.mask !== "thousand") return;
    const cursorEnd = el.selectionEnd;
    const before = el.value;
    const formatted = app.formatThousand(before);
    el.value = formatted;
    // Best-effort cursor restore.
    const diff = formatted.length - before.length;
    if (cursorEnd !== null) {
      const pos = Math.max(0, cursorEnd + diff);
      try {
        el.setSelectionRange(pos, pos);
      } catch (e) {
        /* ignore */
      }
    }
  });

  // ---------- Confirm helper -----------------------------------------------

  app.confirm = function (message, onConfirm) {
    if (window.confirm(message)) {
      if (typeof onConfirm === "function") onConfirm();
      return true;
    }
    return false;
  };

  // HTMX confirm hook: <button hx-confirm="..."> handled natively, ini fallback
  // untuk kasus inline data-confirm.
  document.addEventListener("click", function (event) {
    const el = event.target.closest("[data-confirm]");
    if (!el) return;
    const msg = el.getAttribute("data-confirm");
    if (!msg) return;
    if (!window.confirm(msg)) {
      event.preventDefault();
      event.stopPropagation();
    }
  });

  window.app = app;

  // ---------- Global Search (Alpine component) -----------------------------
  // Dipakai oleh topbar global search. Menyimpan state showResults dan
  // keyboard navigation antar [data-search-hit] di #search-results.
  window.globalSearch = function () {
    return {
      showResults: false,
      hits: [],
      highlight: -1,
      onFocus() {
        if (this.$refs.input.value.trim().length >= 2) {
          this.showResults = true;
        }
      },
      close() {
        this.showResults = false;
        this.highlight = -1;
        this._clearHighlight();
      },
      onResults() {
        const root = document.getElementById("search-results");
        if (!root) return;
        this.hits = Array.from(root.querySelectorAll("[data-search-hit]"));
        this.showResults = root.children.length > 0;
        this.highlight = -1;
      },
      moveHighlight(delta) {
        if (!this.showResults) {
          if (this.$refs.input.value.trim().length >= 2) this.showResults = true;
          return;
        }
        if (this.hits.length === 0) return;
        this.highlight = (this.highlight + delta + this.hits.length) % this.hits.length;
        this._clearHighlight();
        const el = this.hits[this.highlight];
        if (el) {
          el.classList.add("bg-slate-100");
          el.scrollIntoView({ block: "nearest" });
        }
      },
      submitHighlight(event) {
        if (this.highlight >= 0 && this.hits[this.highlight]) {
          event.preventDefault();
          window.location.href = this.hits[this.highlight].getAttribute("href");
          return;
        }
        // Fallback: lihat semua hasil.
        const q = this.$refs.input.value.trim();
        if (q.length >= 2) {
          window.location.href = "/search/all?q=" + encodeURIComponent(q);
        }
      },
      _clearHighlight() {
        this.hits.forEach((el) => el.classList.remove("bg-slate-100"));
      },
    };
  };

  // Keyboard shortcut Ctrl+K / Cmd+K untuk fokus global search.
  document.addEventListener("keydown", function (event) {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
      const input = document.querySelector('input[name="q"][hx-get="/search"]');
      if (input) {
        event.preventDefault();
        input.focus();
        input.select();
      }
    }
  });

  // ---------- DataTable keyboard navigation -------------------------------
  // Pakai sebagai Alpine scope: <table x-data="tableNav('/penjualan/:id')">.
  // Setiap <tr> data row tambah data-id="...". Search input sertakan
  // `data-table-search` untuk fokus saat tekan "/".
  window.tableNav = function (detailURL) {
    return {
      selected: -1,
      _rows() {
        return Array.from(this.$el.querySelectorAll("tr[data-id]"));
      },
      init() {
        this._handler = (e) => {
          const tag = (e.target && e.target.tagName) || "";
          const typing = tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT";
          if (e.key === "/") {
            if (typing) return;
            const sel = this.$el.querySelector("[data-table-search]") ||
                        document.querySelector("[data-table-search]");
            if (sel) {
              sel.focus();
              if (sel.select) sel.select();
              e.preventDefault();
            }
            return;
          }
          if (typing) return;
          if (e.key === "ArrowDown") {
            const rows = this._rows();
            this.selected = Math.min(rows.length - 1, this.selected + 1);
            this._highlight();
            e.preventDefault();
          } else if (e.key === "ArrowUp") {
            this.selected = Math.max(0, this.selected - 1);
            this._highlight();
            e.preventDefault();
          } else if (e.key === "Enter" && this.selected >= 0 && detailURL) {
            const rows = this._rows();
            const id = rows[this.selected] && rows[this.selected].dataset.id;
            if (id) {
              window.location = detailURL.replace(":id", id);
            }
          } else if (e.key === "Escape") {
            this.selected = -1;
            this._highlight();
          }
        };
        window.addEventListener("keydown", this._handler);
      },
      destroy() {
        if (this._handler) window.removeEventListener("keydown", this._handler);
      },
      _highlight() {
        const rows = this._rows();
        rows.forEach((tr, i) => tr.classList.toggle("row-active", i === this.selected));
        if (this.selected >= 0 && rows[this.selected]) {
          rows[this.selected].scrollIntoView({ block: "nearest" });
        }
      },
    };
  };

  // ---------- Bulk select (multi-row) -------------------------------------
  // Alpine scope untuk tabel dengan multi-select checkbox.
  window.bulkSelect = function () {
    return {
      selected: [],
      isAll() {
        const all = this.allIds();
        return all.length > 0 && this.selected.length === all.length;
      },
      allIds() {
        return Array.from(this.$el.querySelectorAll("tr[data-id]")).map((tr) => tr.dataset.id);
      },
      toggleAll(ev) {
        if (ev.target.checked) {
          this.selected = this.allIds();
        } else {
          this.selected = [];
        }
      },
    };
  };

  // bulkSubmit kirim form POST ke `url` dengan `name[]=id` repeated.
  window.bulkSubmit = function (url, name, ids) {
    const f = document.createElement("form");
    f.method = "POST";
    f.action = url;
    ids.forEach((id) => {
      const i = document.createElement("input");
      i.type = "hidden";
      i.name = name + "[]";
      i.value = id;
      f.appendChild(i);
    });
    // The capture-phase submit handler above auto-injects csrf_token, but we
    // attach the form to <body> + submit synchronously, so to be safe (and to
    // not depend on event ordering) we add the hidden field explicitly here.
    const t = app.getCsrfToken();
    if (t) {
      const i = document.createElement("input");
      i.type = "hidden";
      i.name = "csrf_token";
      i.value = t;
      f.appendChild(i);
    }
    document.body.appendChild(f);
    f.submit();
  };

  // ---------- Date Range preset (Alpine component) -------------------------
  // Dipakai @components.DateRange. Tombol preset memanipulasi 2 input date
  // via $refs.from / $refs.to lalu trigger event change supaya parent form
  // (HTMX hx-trigger="change") bisa auto-submit.
  window.dateRange = function () {
    return {
      preset(p) {
        const today = new Date();
        const fmt = (d) => {
          const y = d.getFullYear();
          const m = String(d.getMonth() + 1).padStart(2, "0");
          const day = String(d.getDate()).padStart(2, "0");
          return `${y}-${m}-${day}`;
        };
        const start = new Date(today);
        const end = new Date(today);
        switch (p) {
          case "today":
            break;
          case "7d":
            start.setDate(today.getDate() - 6);
            break;
          case "month":
            start.setDate(1);
            break;
          case "lastmonth":
            start.setMonth(today.getMonth() - 1, 1);
            end.setMonth(today.getMonth(), 0);
            break;
          case "year":
            start.setMonth(0, 1);
            break;
          default:
            break;
        }
        if (this.$refs.from) {
          this.$refs.from.value = fmt(start);
          this.$refs.from.dispatchEvent(new Event("change", { bubbles: true }));
        }
        if (this.$refs.to) {
          this.$refs.to.value = fmt(end);
          this.$refs.to.dispatchEvent(new Event("change", { bubbles: true }));
        }
      },
    };
  };

  // ---------- Notification count (Alpine component) ------------------------
  // Dipakai @components.NotificationBell. Auto-refresh tiap 60 detik.
  window.notifCount = function () {
    return {
      open: false,
      count: 0,
      _timer: null,
      async init() {
        await this.refresh();
        this._timer = setInterval(() => this.refresh(), 60000);
      },
      async refresh() {
        try {
          const r = await fetch("/notifications/count", {
            headers: { Accept: "application/json" },
            credentials: "same-origin",
          });
          if (!r.ok) return;
          const j = await r.json();
          this.count = j.count || 0;
        } catch (e) {
          /* ignore */
        }
      },
    };
  };

  // ---------- Sales chart tooltip (Alpine component) -----------------------
  // Dipakai @components.SalesChart. Hover hotspot rect → baca data-hotspot
  // payload (JSON) → tampilkan tooltip mengikuti pointer.
  window.salesChart = function () {
    return {
      open: false,
      tipX: 0,
      tipY: 0,
      tipDate: "",
      tipRows: [],
      onMove(event) {
        const target = event.target.closest("[data-hotspot]");
        if (!target) {
          this.open = false;
          return;
        }
        const payload = target.getAttribute("data-hotspot");
        if (!payload) return;
        let data;
        try {
          data = JSON.parse(payload);
        } catch (e) {
          return;
        }
        const svg = this.$refs.svg;
        if (!svg) return;
        const rect = svg.getBoundingClientRect();
        const dataX = parseFloat(target.getAttribute("data-x") || "0");
        const vbWidth = (svg.viewBox && svg.viewBox.baseVal && svg.viewBox.baseVal.width) || rect.width;
        const px = (dataX / vbWidth) * rect.width;
        this.tipX = px;
        this.tipY = event.clientY - rect.top;
        this.tipDate = data.d;
        this.tipRows = data.rows || [];
        this.open = true;
      },
      hide() {
        this.open = false;
      },
    };
  };

  // Keyboard shortcut "/" untuk fokus global search (skip kalau sedang
  // typing di field input/textarea/select).
  document.addEventListener("keydown", function (event) {
    if (event.key !== "/") return;
    const tag = (event.target && event.target.tagName) || "";
    if (
      tag === "INPUT" ||
      tag === "TEXTAREA" ||
      tag === "SELECT" ||
      (event.target && event.target.isContentEditable)
    ) {
      return;
    }
    const input = document.querySelector('input[name="q"][hx-get="/search"]');
    if (input) {
      event.preventDefault();
      input.focus();
      input.select();
    }
  });

  // ---------- Online status (Alpine component) ------------------------------
  // Dipakai oleh @components.OnlineStatus(). Hitung pending dari window.tbSync.
  window.onlineStatus = function () {
    return {
      online: navigator.onLine,
      pending: 0,
      hover: false,
      userName: (window.tbUser && window.tbUser.name) || "",
      offlineSince: null,   // timestamp ms saat pertama kali offline
      offlineAgo: "",
      offlineLong: false,   // true bila offline > 5 menit
      _timer: null,
      async init() {
        window.addEventListener("online", () => {
          this.online = true;
          this.offlineSince = null;
          this.offlineLong = false;
          this.offlineAgo = "";
          this.refresh();
        });
        window.addEventListener("offline", () => {
          this.online = false;
          this.offlineSince = Date.now();
          this.tickOffline();
        });
        if (!this.online && !this.offlineSince) {
          this.offlineSince = Date.now();
        }
        window.addEventListener("tbsync:flushed", () => this.refresh());
        await this.refresh();
        this._timer = setInterval(() => {
          this.refresh();
          this.tickOffline();
        }, 5000);
      },
      tickOffline() {
        if (this.online || !this.offlineSince) {
          this.offlineAgo = "";
          this.offlineLong = false;
          return;
        }
        const sec = Math.floor((Date.now() - this.offlineSince) / 1000);
        if (sec < 60) this.offlineAgo = sec + " detik";
        else if (sec < 3600) this.offlineAgo = Math.floor(sec / 60) + " menit";
        else this.offlineAgo = Math.floor(sec / 3600) + " jam";
        this.offlineLong = sec > 300; // 5 menit
      },
      async refresh() {
        if (window.tbSync && typeof window.tbSync.size === "function") {
          try {
            this.pending = await window.tbSync.size();
          } catch (e) {
            this.pending = 0;
          }
        }
      },
      async manualSync() {
        if (window.tbSync && typeof window.tbSync.flush === "function") {
          await window.tbSync.flush();
          await this.refresh();
        }
      },
    };
  };
})();

/*
 * Service worker registration.
 * /sw.js harus diserve dari root path supaya scope = full domain.
 */
(function () {
  "use strict";
  if (!("serviceWorker" in navigator)) return;
  window.addEventListener("load", function () {
    navigator.serviceWorker
      .register("/sw.js", { scope: "/" })
      .catch(function (err) {
        console.warn("SW register failed:", err);
      });
  });
})();

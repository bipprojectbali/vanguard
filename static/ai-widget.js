// Jena AI (BL-162 PoC) — dua hal murni JS yang tak pantas jadi signal Datastar:
// auto-scroll #jena-thread ke bawah tiap SSE PatchElements menambah bubble baru,
// dan tombol Escape menutup panel. Dimuat DEFER (bukan sinkron): state jenaOpen
// mulai false lewat Datastar signal biasa, tak ada risiko FOUC seperti
// sidebar.js/theme.js yang harus set atribut <html> sebelum paint.
(function () {
  "use strict";

  function bind() {
    var thread = document.getElementById("jena-thread");
    if (thread) {
      // SSE PatchElements mode append menyisipkan node baru sebagai child —
      // MutationObserver childList cukup, tak perlu polling.
      var observer = new MutationObserver(function () {
        thread.scrollTop = thread.scrollHeight;
      });
      observer.observe(thread, { childList: true });
    }

    document.addEventListener("keydown", function (e) {
      if (e.key !== "Escape") return;
      var panel = document.getElementById("jena-panel");
      if (!panel) return;
      // Klik tombol tutup yang sudah punya handler data-on (Datastar) — bukan
      // memanipulasi signal langsung dari JS, jaga satu sumber kebenaran state.
      var closeBtn = panel.querySelector('[aria-label="Tutup Jena AI"]');
      if (closeBtn) closeBtn.click();
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", bind);
  } else {
    bind();
  }
})();

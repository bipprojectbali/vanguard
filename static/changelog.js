// Badge "ada pembaruan" untuk tombol Pembaruan di footer sidebar.
//
// File terpisah (bukan inline) agar lolos CSP `script-src 'self'`. Logika:
// bandingkan versi aplikasi (atribut data-app-version pada tombol) dengan versi
// terakhir-dilihat yang tersimpan di localStorage. Beda / belum pernah dilihat →
// tampilkan badge. Klik tombol = menandai versi ini sudah dilihat → badge hilang.
//
// Per-browser (localStorage), tanpa server, tanpa DB. Sama seperti theme.js: bila
// storage tak tersedia (incognito) badge cukup tak tampil — tak menggagalkan apa pun.
(function () {
  "use strict";
  var KEY = "changelog:lastSeen";

  function init() {
    var btn = document.querySelector("[data-changelog-btn]");
    if (!btn) return;

    var version = btn.getAttribute("data-app-version") || "";
    var badge = btn.querySelector("[data-changelog-badge]");

    var seen = null;
    try {
      seen = localStorage.getItem(KEY);
    } catch (e) {
      /* storage tak tersedia — badge tak tampil, ikut default bersih */
    }

    // Tampilkan badge hanya bila ada versi & belum dilihat / versi berubah.
    if (version && seen !== version && badge) {
      badge.classList.remove("hidden");
    }

    // Klik tombol = tandai versi ini sudah dilihat → sembunyikan badge.
    btn.addEventListener("click", function () {
      if (version) {
        try {
          localStorage.setItem(KEY, version);
        } catch (e) {
          /* abaikan bila storage tak tersedia */
        }
      }
      if (badge) badge.classList.add("hidden");
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();

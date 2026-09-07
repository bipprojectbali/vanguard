// phonenum.js — penyaring LIVE input nomor telepon (HP/WhatsApp) bertanda
// [data-phonenum]. Progressive enhancement: saat diketik, karakter yang tak
// diizinkan langsung DIBUANG sehingga field hanya pernah berisi bentuk nomor
// yang sah — tanpa menunggu submit (BL-81). File terpisah (bukan inline) agar
// lolos CSP `script-src 'self'` (gotcha #16); dimuat `defer` (butuh DOM).
//
// Diizinkan: digit 0-9, '+' HANYA sebagai prefix (kode negara "+62"), plus
// spasi & tanda hubung sebagai pemisah kosmetik. Ini CERMIN pattern field
// (`[0-9+ -]*`) & aturan backend optPhone. TIDAK menormalkan ke digit polos:
// backend menerima '+'/spasi/'-', jadi bentuk asli pengguna ("+62 812-…")
// dipertahankan.
//
// BL-2: JANGAN andalkan type="number" (menolak leading zero "0812…" & '+').
// Tanpa JS pun AMAN: backend (optPhone) tetap penolak sesungguhnya saat submit;
// ini murni UX — mencegah karakter mustahil masuk sejak diketik.
(function () {
  "use strict";

  // Buang semua selain digit, '+', spasi, '-'; lalu pertahankan '+' HANYA bila
  // ia karakter pertama (kode negara), buang '+' di posisi lain.
  function sanitize(s) {
    s = (s || "").replace(/[^\d+ -]/g, "");
    var lead = s.charAt(0) === "+" ? "+" : "";
    return lead + s.replace(/\+/g, "");
  }

  // Saring sambil menjaga posisi kursor: hitung berapa karakter "lolos" sebelum
  // kursor, lalu tempatkan kursor setelah sejumlah itu di nilai bersih (kursor
  // tak meloncat ke akhir saat menyunting di tengah).
  function filter(el) {
    var start = el.selectionStart || 0;
    var keptBefore = sanitize(el.value.slice(0, start)).length;
    var next = sanitize(el.value);
    if (next === el.value) return; // tak ada yang dibuang — jangan usik kursor
    el.value = next;
    var pos = keptBefore < next.length ? keptBefore : next.length;
    try {
      el.setSelectionRange(pos, pos);
    } catch (e) {
      /* input non-text (jarang) — abaikan */
    }
  }

  function init() {
    var inputs = document.querySelectorAll("[data-phonenum]");
    for (var i = 0; i < inputs.length; i++) {
      var el = inputs[i];
      if (el.value) el.value = sanitize(el.value); // rapikan nilai prefill
      el.addEventListener("input", (function (node) {
        return function () {
          filter(node);
        };
      })(el));
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();

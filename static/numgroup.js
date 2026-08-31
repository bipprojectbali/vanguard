// numgroup.js — pengelompokan ribuan (gaya Indonesia: titik) untuk input uang
// bertanda [data-numgroup] (mis. Nilai Estimasi lead). Progressive enhancement:
// memformat TAMPILAN saat diketik/dimuat, lalu MENORMALKAN nilainya jadi digit
// polos ("5.000.000" → "5000000") saat form disubmit — sehingga backend selalu
// menerima angka bersih. File terpisah (bukan inline) agar lolos CSP
// `script-src 'self'` (gotcha #16); dimuat `defer` (butuh DOM, bukan pra-paint).
//
// Tanpa JS pun AMAN: backend (cleanThousands) juga membuang pemisah ribuan, jadi
// nilai terformat maupun digit mentah sama-sama diterima. Hanya utk uang BULAT
// (rupiah tanpa sen) — titik diperlakukan sebagai pemisah ribuan, bukan desimal.
(function () {
  "use strict";

  function digitsOnly(s) {
    return (s || "").replace(/\D+/g, "");
  }

  // Sisipkan titik tiap 3 digit dari kanan.
  function group(digits) {
    return digits.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  }

  // Format tampilan sambil menjaga posisi kursor (dihitung dari akhir agar tak
  // meloncat saat titik ditambah/dihapus di kiri kursor).
  function reformat(el) {
    var fromEnd = el.value.length - (el.selectionStart || 0);
    el.value = group(digitsOnly(el.value));
    var pos = el.value.length - fromEnd;
    if (pos < 0) pos = 0;
    try {
      el.setSelectionRange(pos, pos);
    } catch (e) {
      /* input non-text (jarang) — abaikan */
    }
  }

  function bindForm(form) {
    if (!form || form.__numgroupBound) return;
    form.__numgroupBound = true;
    // Native submit (tanpa preventDefault) → PRG 303 tetap jalan; kita cuma
    // menormalkan nilai tepat sebelum data terkirim.
    form.addEventListener("submit", function () {
      var nodes = form.querySelectorAll("[data-numgroup]");
      for (var i = 0; i < nodes.length; i++) {
        nodes[i].value = digitsOnly(nodes[i].value);
      }
    });
  }

  function init() {
    var inputs = document.querySelectorAll("[data-numgroup]");
    for (var i = 0; i < inputs.length; i++) {
      var el = inputs[i];
      if (el.value) el.value = group(digitsOnly(el.value));
      el.addEventListener("input", (function (node) {
        return function () {
          reformat(node);
        };
      })(el));
      bindForm(el.form || el.closest("form"));
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();

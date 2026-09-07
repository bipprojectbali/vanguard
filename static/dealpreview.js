// dealpreview.js — preview MRR/ARR terhitung di form Deal (BL-87 opsi c).
// Menutup ketaksinkronan makna field "Nilai per periode termin": saat deal
// Closed Won, nilai deal diperlakukan PER-TERMIN → MRR = nilai ÷ bulan-kontrak,
// ARR = MRR × 12 (cermin sales_deals_won_subscription.go). Tanpa preview user
// mudah salah input (mis. niat ARR + Termin Monthly → ARR 12× lipat).
//
// Progressive enhancement: membaca nilai + termin, lalu MENAMPILKAN perkiraan.
// Tak mengubah nilai yang ter-submit (backend tetap penjaga). File terpisah
// (bukan inline) agar lolos CSP `script-src 'self'` (gotcha #16); peta
// Termin→bulan ditanam server sebagai <script type="application/json"> (bukan
// hardcode di sini — satu perubahan termContractMonths merambat ke preview).
//
// Tanpa JS / tanpa peta: form tetap jalan; slot preview diam "—" (bukan syarat
// submit). Rupiah dibulatkan ke skala uang lalu desimal dibuang untuk tampilan
// (cermin formatRupiah); nilai tetap "Perkiraan" (pembagian bisa tak bulat).
(function () {
  "use strict";

  var MONTHS_PER_YEAR = 12; // protokol: ARR = MRR × 12 (const monthsPerYear Go).

  // Peta Termin → bulan-kontrak dari data server. null/invalid → preview mati.
  function loadTermMonths() {
    var el = document.getElementById("deal-term-months");
    if (!el) return null;
    try {
      var m = JSON.parse(el.textContent || "null");
      return m && typeof m === "object" ? m : null;
    } catch (e) {
      return null;
    }
  }

  function digitsOnly(s) {
    return (s || "").replace(/\D+/g, "");
  }

  // Sisipkan titik tiap 3 digit dari kanan ("5000000" → "5.000.000").
  function group(digits) {
    return digits.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  }

  // Bulatkan ke 2 desimal (skala uang, cermin NUMERIC(15,2) half-up non-negatif).
  function round2(x) {
    return Math.round(x * 100) / 100;
  }

  // "Rp 5.000.000" — desimal dibuang untuk tampilan (cermin formatRupiah).
  function rupiah(v) {
    return "Rp " + group(String(Math.floor(v)));
  }

  function bindPreview(box, months) {
    var amountEl = document.querySelector(box.getAttribute("data-amount-sel"));
    var termEl = document.querySelector(box.getAttribute("data-term-sel"));
    var mrrEl = box.querySelector("[data-deal-mrr]");
    var arrEl = box.querySelector("[data-deal-arr]");
    var noteEl = box.querySelector("[data-deal-note]");
    if (!amountEl || !termEl || !mrrEl || !arrEl) return;

    var IDLE = "Isi Nilai & pilih Termin Langganan untuk melihat perkiraan MRR/ARR.";

    function clear() {
      mrrEl.textContent = "—";
      arrEl.textContent = "—";
      if (noteEl) noteEl.textContent = IDLE;
    }

    function recompute() {
      var amount = parseInt(digitsOnly(amountEl.value), 10);
      var term = termEl.value;
      var m = months[term];
      if (!amount || amount <= 0 || !term || !m || m <= 0) {
        clear();
        return;
      }
      var mrr = round2(amount / m);
      var arr = round2(mrr * MONTHS_PER_YEAR);
      mrrEl.textContent = rupiah(mrr);
      arrEl.textContent = rupiah(arr);
      if (noteEl) {
        noteEl.textContent =
          "Termin " + term + " · " + m + " bulan kontrak · nilai dianggap per SATU termin.";
      }
    }

    amountEl.addEventListener("input", recompute);
    termEl.addEventListener("change", recompute);
    termEl.addEventListener("input", recompute);
    recompute();
  }

  function init() {
    var months = loadTermMonths();
    if (!months) return; // peta tak ada → biarkan slot "—" (fallback aman).
    var boxes = document.querySelectorAll("[data-deal-preview]");
    for (var i = 0; i < boxes.length; i++) bindPreview(boxes[i], months);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();

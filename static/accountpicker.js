// Pemilih desa induk yang bisa diketik/dicari (BL-5). File terpisah (bukan
// inline) agar lolos CSP script-src 'self' — pola sama regions.js. Native
// <datalist> menyediakan autocomplete browser; skrip ini memetakan nama desa
// yang diketik → id numerik ke input hidden name="account_id" yang ter-submit.
//
// Kontrak markup (dirender contactAccountPickerField, contacts_form.go):
//   [data-account-picker]           wadah satu pemilih
//     [data-account-search]         input teks tampak (list="<datalistId>"), TAK bernama
//     [data-account-value]          input hidden name="account_id" (nilai ter-submit)
//     <datalist id="<datalistId>">  <option value="Nama" data-account-id="123">
//
// Cocok = value opsi sama persis (case-insensitive, di-trim) dgn ketikan.
// Tak cocok → hidden dikosongkan + setCustomValidity → submit ditahan browser.
// Backend tetap penjaga terakhir (loadOwnedAccount → 404 di luar cakupan).
(function () {
  "use strict";

  function ready(fn) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", fn);
    } else {
      fn();
    }
  }

  function initPicker(root) {
    var search = root.querySelector("[data-account-search]");
    var hidden = root.querySelector("[data-account-value]");
    if (!search || !hidden) return;
    var listId = search.getAttribute("list");
    var list = listId ? document.getElementById(listId) : null;
    if (!list) return;

    // sync: cari opsi yang namanya cocok persis dgn ketikan, set id ke hidden.
    function sync() {
      var typed = search.value.trim().toLowerCase();
      var id = "";
      if (typed) {
        var opts = list.querySelectorAll("option");
        for (var i = 0; i < opts.length; i++) {
          if ((opts[i].value || "").trim().toLowerCase() === typed) {
            id = opts[i].getAttribute("data-account-id") || "";
            break;
          }
        }
      }
      hidden.value = id;
      // Ada ketikan tapi tak ada opsi cocok → tandai invalid agar form tak
      // ter-submit dgn desa yang bukan pilihan sah. Kosong dibiarkan ke
      // required bawaan (pesan "wajib diisi" browser).
      search.setCustomValidity(typed && !id ? "Pilih desa dari daftar." : "");
    }

    search.addEventListener("input", sync);
    search.addEventListener("change", sync);
    sync(); // preselect (form re-render) / status awal
  }

  ready(function () {
    var roots = document.querySelectorAll("[data-account-picker]");
    for (var i = 0; i < roots.length; i++) initPicker(roots[i]);
  });
})();

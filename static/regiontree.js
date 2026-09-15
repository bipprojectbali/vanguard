// Cascading dropdown Provinsi → Kabupaten/Kota → Kecamatan tab "Wilayah" modal
// RegionSearchModal (perluasan BL-163, internal/ui/region_search.go
// regionSearchTreePanel). BEDA dengan regions.js: modal ini dirender di
// SETIAP halaman lewat AppShell, jadi ketiga level di-LAZY-FETCH satu per
// satu (bukan embed dataset ~7.817 baris sekali muat) lewat
// data-tree-url (wsBase + "/regions/tree", handler regions_tree.go).
// File terpisah (bukan inline) agar lolos CSP script-src 'self'.
//
// Pencarian akhir (setelah Kecamatan dipilih) DITANGANI Datastar langsung
// lewat atribut data-on:change yang sudah ditulis Go pada <select
// data-tree-level="3">, BUKAN di sini — JS ini HANYA mengisi <option> &
// enable/disable select berikutnya (satu paradigma interaktivitas,
// CLAUDE.md §Batasan).
(function () {
  "use strict";

  function ready(fn) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", fn);
    } else {
      fn();
    }
  }

  // populateSelect mengisi ulang <select> dgn placeholder + opsi lalu
  // aktifkan/nonaktifkan — sama pola dgn regions.js, disalin (bukan
  // di-share) supaya kedua file tetap independen (grup atribut beda,
  // tak ada risiko silang).
  function populateSelect(sel, placeholder, rows, enable) {
    while (sel.firstChild) sel.firstChild.remove();
    var ph = document.createElement("option");
    ph.value = "";
    ph.textContent = placeholder;
    sel.appendChild(ph);
    for (var i = 0; i < rows.length; i++) {
      var opt = document.createElement("option");
      opt.value = String(rows[i].id);
      opt.textContent = rows[i].name;
      sel.appendChild(opt);
    }
    sel.disabled = !enable;
    sel.value = "";
  }

  // fetchLevel mem-fetch satu level (1/2/3) dari treeUrl, opsional
  // parent=<id> (level 2 & 3). Gagal fetch → callback dgn array kosong
  // (dropdown kosong+nonaktif; backend tetap penjaga sebenarnya).
  function fetchLevel(treeUrl, level, parentId, cb) {
    var url = treeUrl + "?level=" + level;
    if (parentId) url += "&parent=" + encodeURIComponent(parentId);
    fetch(url, { headers: { Accept: "application/json" }, credentials: "same-origin" })
      .then(function (res) { return res.ok ? res.json() : []; })
      .then(function (rows) { cb(rows || []); })
      .catch(function () { cb([]); });
  }

  function initGroup(container) {
    var treeUrl = container.getAttribute("data-tree-url") || "";
    var sel1 = container.querySelector('[data-tree-level="1"]');
    var sel2 = container.querySelector('[data-tree-level="2"]');
    var sel3 = container.querySelector('[data-tree-level="3"]');
    if (!treeUrl || !sel1 || !sel2 || !sel3) return;

    sel1.addEventListener("change", function () {
      populateSelect(sel2, "— Pilih Kabupaten/Kota —", [], false);
      populateSelect(sel3, "— Pilih Kecamatan —", [], false);
      if (!sel1.value) return;
      populateSelect(sel2, "Memuat…", [], false);
      fetchLevel(treeUrl, 2, sel1.value, function (rows) {
        populateSelect(sel2, "— Pilih Kabupaten/Kota —", rows, true);
      });
    });

    sel2.addEventListener("change", function () {
      populateSelect(sel3, "— Pilih Kecamatan —", [], false);
      if (!sel2.value) return;
      populateSelect(sel3, "Memuat…", [], false);
      fetchLevel(treeUrl, 3, sel2.value, function (rows) {
        populateSelect(sel3, "— Pilih Kecamatan —", rows, true);
      });
    });

    // Kecamatan (sel3) TIDAK diberi listener di sini — trigger pencarian
    // sudah atribut Datastar data-on:change (Go), JS tak boleh dobel-pasang.

    fetchLevel(treeUrl, 1, null, function (rows) {
      populateSelect(sel1, "— Pilih Provinsi —", rows, true);
    });
  }

  ready(function () {
    var groups = document.querySelectorAll("[data-tree-group]");
    for (var i = 0; i < groups.length; i++) initGroup(groups[i]);
  });
})();

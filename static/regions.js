// Cascading dropdown wilayah administratif (Provinsi → Kabupaten/Kota →
// Kecamatan), ADR 0009. File terpisah (bukan inline) agar lolos CSP
// script-src 'self'. Dataset PENUH (~7.817 baris) ditanam server sebagai
// <script type="application/json" data-region-json="<embedID>"> (JSON TIDAK
// dieksekusi browser → CSP-safe). Kita JSON.parse sekali per grup, lalu
// populate 3 <select> murni di sini — TANPA round-trip server per pilihan
// (lihat internal/ui/pages/panel/regionselect.go).
//
// Hanya <select data-region-level="3"> (Kecamatan) yang benar-benar bernama
// (name="district_id") & ter-submit; level 1 & 2 cuma alat bantu filter.
(function () {
  "use strict";

  function ready(fn) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", fn);
    } else {
      fn();
    }
  }

  // populateSelect mengisi ulang <select> dgn placeholder + opsi, lalu set
  // value terpilih (jika ada di antara opsi) & aktifkan/nonaktifkan.
  function populateSelect(sel, placeholder, options, selectedId, enable) {
    while (sel.firstChild) sel.firstChild.remove();
    var ph = document.createElement("option");
    ph.value = "";
    ph.textContent = placeholder;
    sel.appendChild(ph);
    for (var i = 0; i < options.length; i++) {
      var opt = document.createElement("option");
      opt.value = String(options[i].id);
      opt.textContent = options[i].name;
      sel.appendChild(opt);
    }
    sel.disabled = !enable;
    sel.value = selectedId != null ? String(selectedId) : "";
  }

  // initGroup menyiapkan satu grup (satu instance regionSelect): parse JSON,
  // bangun peta id→node + anak-per-induk, lalu populate 3 select-nya —
  // termasuk preselect 3 level sekaligus bila select Kecamatan sudah punya
  // data-selected-value (form edit).
  function initGroup(embedId, raw) {
    var rows;
    try {
      rows = JSON.parse(raw);
    } catch (e) {
      return; // JSON rusak — jangan gagalkan form, dropdown tetap kosong
    }
    if (!rows || !rows.length) return;

    var byId = {};
    var childrenOf = {}; // parent_id -> [node,...]
    var provinces = [];
    for (var i = 0; i < rows.length; i++) {
      var r = rows[i];
      byId[r.id] = r;
      if (r.level === 1) {
        provinces.push(r);
      } else if (r.parent_region_id != null) {
        var key = String(r.parent_region_id);
        if (!childrenOf[key]) childrenOf[key] = [];
        childrenOf[key].push(r);
      }
    }

    var sel1 = document.querySelector(
      '[data-region-group="' + embedId + '"][data-region-level="1"]'
    );
    var sel2 = document.querySelector(
      '[data-region-group="' + embedId + '"][data-region-level="2"]'
    );
    var sel3 = document.querySelector(
      '[data-region-group="' + embedId + '"][data-region-level="3"]'
    );
    if (!sel1 || !sel2 || !sel3) return;

    function onLevel1Change() {
      var pid = sel1.value;
      populateSelect(sel2, "— Pilih Kabupaten/Kota —", pid ? childrenOf[pid] || [] : [], null, !!pid);
      populateSelect(sel3, "— Pilih Kecamatan —", [], null, false);
    }
    function onLevel2Change() {
      var pid = sel2.value;
      populateSelect(sel3, "— Pilih Kecamatan —", pid ? childrenOf[pid] || [] : [], null, !!pid);
    }
    sel1.addEventListener("change", onLevel1Change);
    sel2.addEventListener("change", onLevel2Change);

    var selectedDistrictId = sel3.getAttribute("data-selected-value") || "";
    var district = selectedDistrictId ? byId[selectedDistrictId] : null;
    var regency = district ? byId[district.parent_region_id] : null;
    var province = regency ? byId[regency.parent_region_id] : null;

    populateSelect(sel1, "— Pilih Provinsi —", provinces, province ? province.id : null, true);
    if (province) {
      populateSelect(sel2, "— Pilih Kabupaten/Kota —", childrenOf[String(province.id)] || [], regency ? regency.id : null, true);
    }
    if (regency) {
      populateSelect(sel3, "— Pilih Kecamatan —", childrenOf[String(regency.id)] || [], district ? district.id : null, true);
    }
  }

  ready(function () {
    var embeds = document.querySelectorAll("script[data-region-json]");
    for (var i = 0; i < embeds.length; i++) {
      initGroup(embeds[i].getAttribute("data-region-json"), embeds[i].textContent);
    }
  });
})();

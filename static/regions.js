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
//
// BL-66 (form desa saja): bila ada <select data-region-level="4"> (Desa/
// Kelurahan, name="village_id"), level 4 di-LAZY-FETCH per Kecamatan lewat
// data-villages-url (dataset ~83.762 Desa terlalu besar utk diembed). Di form
// desa, level 4 inilah sumber kebenaran (village_id → village_code Kemendagri);
// level 3 turun pangkat jadi filter. Fetch same-origin JSON → lolos CSP.
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

    // sel4 (Desa/Kelurahan) OPSIONAL — hanya form desa (BL-66) yang punya level 4.
    // Absen → seluruh blok Desa jadi no-op (form Leads/Convert tak terpengaruh).
    var sel4 = document.querySelector(
      '[data-region-group="' + embedId + '"][data-region-level="4"]'
    );
    var villagesUrl = sel4 ? sel4.getAttribute("data-villages-url") || "" : "";

    // clearVillages mengosongkan & menonaktifkan dropdown Desa — dipakai saat
    // pilihan di atas Desa berubah (Kecamatan lama tak lagi relevan).
    function clearVillages() {
      if (sel4) populateSelect(sel4, "— Pilih Desa/Kelurahan —", [], null, false);
    }

    // loadVillages me-lazy-fetch Desa satu Kecamatan (dataset penuh terlalu besar
    // utk embed) lalu populate sel4. selectedVillageId (opsional, form edit)
    // dipilih setelah opsi termuat. Gagal fetch → dropdown kosong+nonaktif (form
    // tetap jalan; backend penjaga sebenarnya).
    function loadVillages(districtId, selectedVillageId) {
      if (!sel4 || !districtId || !villagesUrl) return;
      populateSelect(sel4, "Memuat…", [], null, false);
      var sep = villagesUrl.indexOf("?") === -1 ? "?" : "&";
      var url = villagesUrl + sep + "district=" + encodeURIComponent(districtId);
      fetch(url, { headers: { Accept: "application/json" }, credentials: "same-origin" })
        .then(function (res) { return res.ok ? res.json() : []; })
        .then(function (rows) {
          populateSelect(sel4, "— Pilih Desa/Kelurahan —", rows || [], selectedVillageId != null ? selectedVillageId : null, true);
        })
        .catch(function () {
          populateSelect(sel4, "— Pilih Desa/Kelurahan —", [], null, false);
        });
    }

    function onLevel1Change() {
      var pid = sel1.value;
      populateSelect(sel2, "— Pilih Kabupaten/Kota —", pid ? childrenOf[pid] || [] : [], null, !!pid);
      populateSelect(sel3, "— Pilih Kecamatan —", [], null, false);
      clearVillages();
    }
    function onLevel2Change() {
      var pid = sel2.value;
      populateSelect(sel3, "— Pilih Kecamatan —", pid ? childrenOf[pid] || [] : [], null, !!pid);
      clearVillages();
    }
    function onLevel3Change() {
      clearVillages();
      if (sel3.value) loadVillages(sel3.value, null);
    }
    sel1.addEventListener("change", onLevel1Change);
    sel2.addEventListener("change", onLevel2Change);
    if (sel4) sel3.addEventListener("change", onLevel3Change);

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

    // BL-66: form edit — setelah Kecamatan terpreselect, lazy-fetch Desa &
    // preselect village tersimpan (data-selected-value pada sel4). Desa legacy
    // yang village_id-nya kosong → dropdown termuat tanpa terpilih.
    if (sel4 && district) {
      var selectedVillageId = sel4.getAttribute("data-selected-value") || "";
      loadVillages(district.id, selectedVillageId || null);
    }
  }

  ready(function () {
    var embeds = document.querySelectorAll("script[data-region-json]");
    for (var i = 0; i < embeds.length; i++) {
      initGroup(embeds[i].getAttribute("data-region-json"), embeds[i].textContent);
    }
  });
})();

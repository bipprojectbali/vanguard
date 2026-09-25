// dealboard.js — drag-drop & multiselect papan Kanban Deal (BL-75). File
// terpisah (CSP script-src 'self', BUKAN Datastar — spec BL-75 eksplisit
// melarang Datastar untuk fitur ini, beda dari dealStageModal/dealStageControl
// single-deal yang boleh Datastar). Tak ada atribut onclick/inline lain —
// script-src TANPA unsafe-inline (internal/mw/security.go), semua listener
// dipasang di sini via addEventListener.
//
// Kontrak markup (internal/ui/pages/panel/sales_deals_body.go &
// sales_deals_board_bulk.go — ubah BERSAMAAN, jangan sendiri-sendiri):
//   [data-stage-column]              kolom/drop-target, value = nama stage;
//                                    data-col-state (diset JS) = "wide"|"min"
//                                    saat bukan default (lihat blok Kolom).
//   [data-deal-id] (kartu <a>)       draggable=true; data-deal-stage (asal);
//                                    data-deal-name; data-quote-accepted="1"
//                                    (opsional, gate drag Negotiation→Won).
//   [data-col-body]                  kontainer kartu dalam kolom; disembunyikan
//                                    saat data-col-state="min".
//   [data-col-header-full]           header normal/wide (nama tahap+badge+dua
//                                    tombol); disembunyikan saat state "min"
//                                    (56px terlalu sempit utk baris ini).
//   [data-col-header-min]            header pengganti KHUSUS state "min":
//                                    badge count + SATU tombol (aksi "min",
//                                    klik lagi = balik "normal"), vertikal.
//   [data-col-action=wide|min]       tombol expand/minimize per kolom (header;
//                                    "min" muncul di header-full DAN header-min).
//   #deal-select-toggle              tombol toggle mode Pilih (klik-biasa jadi
//                                    seleksi, bukan navigasi) — alternatif
//                                    Ctrl/Cmd/Shift-klik.
//   #deal-select-clear               tombol kosongkan seleksi aktif.
//   #deal-select-count               teks "N dipilih" — JS isi/toggle tampil.
//   #deal-stage-rules                <script type=application/json> map
//                                    stage→[nextStage,...] (dealStageRulesJSON).
//   #deal-stage-bulk-form            <form method=post> target submit.
//   #deal-stage-bulk-ids             kontainer kosong; diisi <input type=hidden
//                                    name=deal_id> per deal terpilih.
//   #deal-stage-bulk-stage           hidden input name=stage (tujuan).
//   #deal-stage-bulk-mode            hidden input name=mode (shared|individual).
//   #deal-stage-bulk-modal           modal terminal (Won/Lost), toggle
//                                    style.display, BUKAN data.Show.
//   [name=ui-mode]                   radio shared/individual.
//   #deal-stage-bulk-shared-fields   field mode shared.
//   #deal-stage-bulk-individual-rows kontainer baris ter-clone mode individual.
//   #deal-stage-bulk-row-template    <template> baris per-deal (mode individual).
//   [data-stage-section=won|lost]    field relevan HANYA target terminal itu.
//   [data-bulk-cancel]               tombol tutup modal tanpa submit (X, Batal).
//   #deal-stage-bulk-summary         teks ringkas "N deal → <stage>".
//
// Seleksi kartu: KLIK BIASA tetap navigasi ke detail (fallback non-drag utuh —
// kontrol native ada di halaman detail), KECUALI mode Pilih aktif (tombol
// #deal-select-toggle) — saat itu klik biasa pun toggle seleksi. Ctrl/Cmd/
// Shift-klik SELALU toggle seleksi multi-kartu tanpa navigasi (pola
// file-manager standar), terlepas dari mode Pilih. Klik-pilih kartu di stage
// BEDA dari seleksi aktif me-reset seleksi ke kartu itu saja (keputusan (a)
// BL-75: multiselect HANYA dalam satu stage sumber). Drag kartu TUNGGAL tanpa
// pra-seleksi tetap jalan: dragstart mereset seleksi ke kartu yang di-drag
// bila kartu itu belum masuk seleksi aktif.
//
// Kolom expand/minimize: state per-stage disimpan localStorage (key
// "dealboard.colstate", JSON {stage: "wide"|"min"}) — pola sama sidebar.js
// (try/catch, storage opsional). Klik tombol yg sudah aktif = kembali ke
// lebar normal (toggle), bukan tumpuk mode.
(function () {
  "use strict";

  var TERMINAL = { "Closed Won": true, "Closed Lost": true };
  var SEL_CLASSES = ["border-primary", "bg-primary/10"];
  var TARGET_CLASSES = ["bg-primary/10"];
  var COL_WIDTH_CLASSES = ["w-64", "w-80", "w-14"];
  var COL_STATE_KEY = "dealboard.colstate";

  function ready(fn) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", fn);
    } else {
      fn();
    }
  }

  ready(function () {
    if (!document.querySelector("[data-stage-column]")) return; // view Tabel: tak ada papan

    var rules = {};
    var rulesEl = document.getElementById("deal-stage-rules");
    if (rulesEl) {
      try {
        rules = JSON.parse(rulesEl.textContent || "{}");
      } catch (e) {
        rules = {};
      }
    }

    var form = document.getElementById("deal-stage-bulk-form");
    var idsBox = document.getElementById("deal-stage-bulk-ids");
    var stageInput = document.getElementById("deal-stage-bulk-stage");
    var modeInput = document.getElementById("deal-stage-bulk-mode");
    var modal = document.getElementById("deal-stage-bulk-modal");
    var summaryEl = document.getElementById("deal-stage-bulk-summary");
    var sharedFields = document.getElementById("deal-stage-bulk-shared-fields");
    var individualRows = document.getElementById("deal-stage-bulk-individual-rows");
    var rowTemplate = document.getElementById("deal-stage-bulk-row-template");
    if (!form || !idsBox || !stageInput || !modeInput || !modal) return;

    // currentStage = tujuan terminal aktif saat modal terbuka. toggleStageSections
    // (won/lost) HARUS dijalankan ulang tiap kali baris individual di-clone
    // (buildIndividualRows) — clone [data-stage-section] baru TIDAK mewarisi
    // hasil toggle sebelumnya (template asalnya tak pernah disentuh
    // querySelectorAll, sebab <template> inert/di luar DOM live). Tanpa ini,
    // mode "Alasan per-deal" selalu menampilkan SEMUA field (won+lost
    // sekaligus) walau target cuma salah satu — bug dilaporkan user.
    var currentStage = null;

    var selectToggleBtn = document.getElementById("deal-select-toggle");
    var selectClearBtn = document.getElementById("deal-select-clear");
    var selectCountEl = document.getElementById("deal-select-count");

    // --- Seleksi ---
    var selected = []; // [{id, stage, name, quoteAccepted}], urutan klik/drag
    var selectedStage = null;
    var selectMode = false; // toggle #deal-select-toggle: klik biasa jadi seleksi

    function isSelected(id) {
      for (var i = 0; i < selected.length; i++) {
        if (selected[i].id === id) return true;
      }
      return false;
    }

    function cardEl(id) {
      return document.querySelector('[data-deal-id="' + id + '"]');
    }

    function applySelectionVisual() {
      var cards = document.querySelectorAll("[data-deal-id]");
      for (var i = 0; i < cards.length; i++) {
        cards[i].classList.remove.apply(cards[i].classList, SEL_CLASSES);
      }
      for (var j = 0; j < selected.length; j++) {
        var el = cardEl(selected[j].id);
        if (el) el.classList.add.apply(el.classList, SEL_CLASSES);
      }
    }

    function updateSelectUI() {
      var n = selected.length;
      if (selectCountEl) {
        selectCountEl.textContent = n > 0 ? n + " dipilih" : "";
        selectCountEl.style.display = n > 0 ? "" : "none";
      }
      if (selectClearBtn) selectClearBtn.style.display = n > 0 ? "" : "none";
    }

    function setSelectMode(on) {
      selectMode = on;
      if (selectToggleBtn) {
        selectToggleBtn.classList.toggle("btn-primary", on);
        selectToggleBtn.classList.toggle("btn-outline", !on);
      }
    }

    function setSelection(items) {
      selected = items;
      selectedStage = items.length ? items[0].stage : null;
      applySelectionVisual();
      updateSelectUI();
    }

    function cardInfo(card) {
      return {
        id: card.getAttribute("data-deal-id"),
        stage: card.getAttribute("data-deal-stage"),
        name: card.getAttribute("data-deal-name") || "",
        quoteAccepted: card.getAttribute("data-quote-accepted") === "1",
      };
    }

    function toggleSelect(card) {
      var info = cardInfo(card);
      if (selectedStage && info.stage !== selectedStage) {
        setSelection([info]);
        return;
      }
      if (isSelected(info.id)) {
        setSelection(
          selected.filter(function (it) {
            return it.id !== info.id;
          })
        );
      } else {
        setSelection(selected.concat([info]));
      }
    }

    document.addEventListener("click", function (e) {
      var card = e.target.closest ? e.target.closest("[data-deal-id]") : null;
      if (card) {
        if (e.ctrlKey || e.metaKey || e.shiftKey || selectMode) {
          e.preventDefault();
          toggleSelect(card);
        }
        return;
      }
      // Modal: klik tepat pada backdrop (bukan bubbling dari isi kartu) = tutup.
      if (e.target === modal) {
        closeModal();
        return;
      }
      var cancel = e.target.closest ? e.target.closest("[data-bulk-cancel]") : null;
      if (cancel) {
        e.preventDefault();
        closeModal();
      }
    });

    if (selectToggleBtn) {
      selectToggleBtn.addEventListener("click", function () {
        setSelectMode(!selectMode);
      });
    }
    if (selectClearBtn) {
      selectClearBtn.addEventListener("click", function () {
        setSelection([]);
      });
    }

    // --- Drag ---
    document.addEventListener("dragstart", function (e) {
      var card = e.target.closest ? e.target.closest("[data-deal-id]") : null;
      if (!card) return;
      var info = cardInfo(card);
      if (!isSelected(info.id)) setSelection([info]);
      if (e.dataTransfer) {
        e.dataTransfer.effectAllowed = "move";
        e.dataTransfer.setData("text/plain", info.id);
      }
    });

    function isValidTarget(targetStage) {
      if (!selectedStage || !selected.length || targetStage === selectedStage) return false;
      var next = rules[selectedStage] || [];
      var ok = false;
      for (var i = 0; i < next.length; i++) {
        if (next[i] === targetStage) {
          ok = true;
          break;
        }
      }
      if (!ok) return false;
      if (targetStage === "Closed Won") {
        for (var j = 0; j < selected.length; j++) {
          if (!selected[j].quoteAccepted) return false;
        }
      }
      return true;
    }

    // dragenter DIPREVENT-DEFAULT juga (bukan cuma dragover): sebagian browser
    // (khususnya Firefox) menolak drop bila hanya dragover yang di-preventDefault
    // — akar bug "bisa drag tapi tidak bisa drop" yang dilaporkan user, SELAIN
    // fix items-stretch di dealKanban (lihat sales_deals_body.go).
    document.addEventListener("dragenter", function (e) {
      var col = e.target.closest ? e.target.closest("[data-stage-column]") : null;
      if (!col) return;
      if (!isValidTarget(col.getAttribute("data-stage-column"))) return;
      e.preventDefault();
    });

    document.addEventListener("dragover", function (e) {
      var col = e.target.closest ? e.target.closest("[data-stage-column]") : null;
      if (!col) return;
      if (!isValidTarget(col.getAttribute("data-stage-column"))) return;
      e.preventDefault();
      if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
      col.classList.add.apply(col.classList, TARGET_CLASSES);
    });

    document.addEventListener("dragleave", function (e) {
      var col = e.target.closest ? e.target.closest("[data-stage-column]") : null;
      if (!col) return;
      if (e.relatedTarget && col.contains(e.relatedTarget)) return;
      col.classList.remove.apply(col.classList, TARGET_CLASSES);
    });

    document.addEventListener("drop", function (e) {
      var col = e.target.closest ? e.target.closest("[data-stage-column]") : null;
      if (!col) return;
      col.classList.remove.apply(col.classList, TARGET_CLASSES);
      var stage = col.getAttribute("data-stage-column");
      if (!isValidTarget(stage)) return;
      e.preventDefault();
      beginMove(stage);
    });

    // --- Submit langsung (non-terminal) / buka modal (terminal) ---
    function fillIds() {
      while (idsBox.firstChild) idsBox.firstChild.remove();
      for (var i = 0; i < selected.length; i++) {
        var input = document.createElement("input");
        input.type = "hidden";
        input.name = "deal_id";
        input.value = selected[i].id;
        idsBox.appendChild(input);
      }
    }

    function beginMove(stage) {
      fillIds();
      stageInput.value = stage;
      if (!TERMINAL[stage]) {
        modeInput.value = "shared";
        form.submit();
        return;
      }
      openModal(stage);
    }

    // --- Modal terminal ---
    function toggleStageSections(stage) {
      var won = stage === "Closed Won";
      var lost = stage === "Closed Lost";
      var wonEls = document.querySelectorAll('[data-stage-section="won"]');
      for (var i = 0; i < wonEls.length; i++) wonEls[i].style.display = won ? "" : "none";
      var lostEls = document.querySelectorAll('[data-stage-section="lost"]');
      for (var j = 0; j < lostEls.length; j++) lostEls[j].style.display = lost ? "" : "none";
    }

    function setFieldsEnabled(container, enabled) {
      if (!container) return;
      var fields = container.querySelectorAll("input, select, textarea");
      for (var i = 0; i < fields.length; i++) fields[i].disabled = !enabled;
    }

    function buildIndividualRows() {
      if (!individualRows || !rowTemplate || !rowTemplate.content) return;
      while (individualRows.firstChild) individualRows.firstChild.remove();
      for (var i = 0; i < selected.length; i++) {
        var item = selected[i];
        var frag = rowTemplate.content.cloneNode(true);
        var label = frag.querySelector(".deal-stage-bulk-row-label");
        if (label) label.textContent = item.name || "Deal #" + item.id;
        var fields = frag.querySelectorAll("[name]");
        for (var f = 0; f < fields.length; f++) {
          fields[f].name = fields[f].name + "__" + item.id;
        }
        individualRows.appendChild(frag);
      }
    }

    function setMode(mode) {
      modeInput.value = mode;
      var radios = document.querySelectorAll('input[name="ui-mode"]');
      for (var i = 0; i < radios.length; i++) radios[i].checked = radios[i].value === mode;
      var shared = mode === "shared";
      if (sharedFields) {
        sharedFields.style.display = shared ? "" : "none";
        setFieldsEnabled(sharedFields, shared);
      }
      if (!shared) buildIndividualRows();
      if (individualRows) {
        individualRows.style.display = shared ? "none" : "";
        setFieldsEnabled(individualRows, !shared);
      }
      // Re-terapkan won/lost per blok yang SEKARANG live di DOM (shared ATAU
      // baris individual baru saja di-clone) — lihat catatan currentStage di atas.
      if (currentStage) toggleStageSections(currentStage);
    }

    document.addEventListener("change", function (e) {
      if (e.target && e.target.name === "ui-mode") setMode(e.target.value);
    });

    function openModal(stage) {
      currentStage = stage;
      if (summaryEl) summaryEl.textContent = selected.length + " deal → " + stage;
      setMode("shared"); // setMode sendiri memanggil toggleStageSections(currentStage)
      modal.style.display = "flex";
    }

    function closeModal() {
      modal.style.display = "none";
    }

    // --- Kolom: expand (wide) / minimize (min) ---
    function loadColState() {
      try {
        return JSON.parse(localStorage.getItem(COL_STATE_KEY) || "{}");
      } catch (e) {
        return {};
      }
    }

    function saveColState(state) {
      try {
        localStorage.setItem(COL_STATE_KEY, JSON.stringify(state));
      } catch (e) {
        /* storage tak tersedia — abaikan, state hanya berlaku sesi ini */
      }
    }

    var colState = loadColState();

    function applyColState(col, state) {
      col.classList.remove.apply(col.classList, COL_WIDTH_CLASSES);
      var body = col.querySelector("[data-col-body]");
      var headerFull = col.querySelector("[data-col-header-full]");
      var headerMin = col.querySelector("[data-col-header-min]");
      if (state === "wide") {
        col.classList.add("w-80");
        if (body) body.style.display = "";
        if (headerFull) headerFull.style.display = "";
        if (headerMin) headerMin.style.display = "none";
      } else if (state === "min") {
        col.classList.add("w-14");
        if (body) body.style.display = "none";
        if (headerFull) headerFull.style.display = "none";
        if (headerMin) headerMin.style.display = "flex";
      } else {
        col.classList.add("w-64");
        if (body) body.style.display = "";
        if (headerFull) headerFull.style.display = "";
        if (headerMin) headerMin.style.display = "none";
        state = "normal";
      }
      col.setAttribute("data-col-state", state);
    }

    var allCols = document.querySelectorAll("[data-stage-column]");
    for (var ci = 0; ci < allCols.length; ci++) {
      var stageName = allCols[ci].getAttribute("data-stage-column");
      if (colState[stageName]) applyColState(allCols[ci], colState[stageName]);
    }

    document.addEventListener("click", function (e) {
      var btn = e.target.closest ? e.target.closest("[data-col-action]") : null;
      if (!btn) return;
      e.preventDefault();
      var col = btn.closest("[data-stage-column]");
      if (!col) return;
      var stageName = col.getAttribute("data-stage-column");
      var action = btn.getAttribute("data-col-action");
      var current = col.getAttribute("data-col-state") || "normal";
      var next = current === action ? "normal" : action; // klik ulang = kembali normal
      applyColState(col, next);
      if (next === "normal") delete colState[stageName];
      else colState[stageName] = next;
      saveColState(colState);
    });
  });
})();

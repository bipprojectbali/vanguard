// Jena AI (BL-162 PoC, riwayat BL-179) — hal murni JS yang tak pantas jadi
// signal Datastar: auto-scroll #jena-thread, Escape menutup panel, DAN
// (BL-179) riwayat percakapan per-tab via sessionStorage — app ini bukan SPA
// (navigasi = full page reload, gotcha #16), jadi DOM/state Datastar selalu
// dibuang; sessionStorage adalah satu-satunya yang bertahan lintas halaman
// tanpa jadi cross-tab/cross-user (beda dari localStorage). Dimuat DEFER
// (bukan sinkron): tak ada risiko FOUC seperti sidebar.js/theme.js yang harus
// set atribut <html> sebelum paint.
(function () {
  "use strict";

  // JENA_HISTORY_MAX HARUS sama dengan jenaMaxHistoryTurns di
  // internal/handler/jena_ai_history.go (keputusan user 28 Sep: 5 giliran
  // terakhir) — batas di sini murni utk hemat storage/payload, backend tetap
  // menegakkan ulang batas yang sama krn field ini client-controlled.
  var JENA_HISTORY_MAX = 5;

  // storageKey menyekat riwayat PER WORKSPACE (bukan cuma per tab) — pathname
  // /w/{slug}/... memuat slug tenant; dua workspace beda di tab/sessionStorage
  // yang sama (jarang tapi mungkin lewat navigasi manual) tak boleh bocor
  // riwayat satu sama lain.
  function storageKey() {
    var m = /^\/w\/[^/]+/.exec(window.location.pathname);
    return "jena-history:" + (m ? m[0] : "default");
  }

  // loadHistory/saveHistory dibungkus try/catch SENGAJA (bukan empty-catch
  // penelan bug) — sessionStorage bisa throw di private browsing/quota penuh;
  // riwayat cuma penambah konteks, bukan syarat inti fitur chat, jadi gagal
  // diam-diam ke riwayat kosong lebih baik drpd mematikan seluruh widget.
  function loadHistory() {
    try {
      var raw = window.sessionStorage.getItem(storageKey());
      if (!raw) return [];
      var parsed = JSON.parse(raw);
      if (!Array.isArray(parsed)) return [];
      return parsed;
    } catch (e) {
      return [];
    }
  }

  function saveHistory(history) {
    try {
      window.sessionStorage.setItem(storageKey(), JSON.stringify(history));
    } catch (e) {
      // Diam-diam gagal (lihat komentar loadHistory) — riwayat tak tersimpan
      // giliran ini, tapi chat tetap berfungsi.
    }
  }

  // syncHistoryField menulis riwayat terkini ke hidden input yang dibaca
  // Datastar @post {contentType:'form'} saat submit (internal/ui/jena_ai.go).
  function syncHistoryField(history) {
    var field = document.getElementById("jena-history-field");
    if (field) field.value = JSON.stringify(history);
  }

  // renderBubble membangun DOM bubble PERSIS kelas yang dipakai
  // JenaAIMessagePair (internal/ui/jena_ai.go) — via textContent (BUKAN
  // innerHTML) supaya teks riwayat yang di-load ulang dari sessionStorage tak
  // pernah dieksekusi sbg markup (sama semangat gotcha #15, sisi klien).
  function renderBubble(question, answer) {
    var frag = document.createDocumentFragment();

    var qWrap = document.createElement("div");
    qWrap.className = "chat chat-end jena-bubble-in";
    var qBubble = document.createElement("div");
    qBubble.className = "chat-bubble chat-bubble-primary";
    qBubble.textContent = question;
    qWrap.appendChild(qBubble);
    frag.appendChild(qWrap);

    var aWrap = document.createElement("div");
    aWrap.className = "chat chat-start jena-bubble-in";
    var aBubble = document.createElement("div");
    aBubble.className = "chat-bubble whitespace-pre-line";
    aBubble.textContent = answer;
    aWrap.appendChild(aBubble);
    frag.appendChild(aWrap);

    return frag;
  }

  // extractTurn membaca {q,a} dari satu node .jena-turn-data (ditanam handler
  // via ui.jenaTurnData, JSON di dalam <script type="application/json">).
  function extractTurn(node) {
    try {
      var parsed = JSON.parse(node.textContent);
      if (typeof parsed.q !== "string" || typeof parsed.a !== "string") {
        return null;
      }
      return { q: parsed.q, a: parsed.a };
    } catch (e) {
      return null;
    }
  }

  // rehydrate menyisipkan bubble riwayat dari sessionStorage SEBELUM
  // #jena-pending (jangkar ujung thread, sama posisi penyisipan giliran baru
  // di handler) — dipanggil SEBELUM MutationObserver dipasang: bubble hasil
  // rehydrate tak punya node .jena-turn-data, jadi tak pernah tertangkap
  // ulang sbg "giliran baru".
  function rehydrate(thread, pending, history) {
    for (var i = 0; i < history.length; i++) {
      var t = history[i];
      if (!t || typeof t.q !== "string" || typeof t.a !== "string") continue;
      thread.insertBefore(renderBubble(t.q, t.a), pending);
    }
  }

  function bind() {
    var thread = document.getElementById("jena-thread");
    var pending = document.getElementById("jena-pending");
    var history = loadHistory();

    if (thread && pending) {
      rehydrate(thread, pending, history);
    }
    // Field disinkron segera setelah rehydrate — giliran berikutnya yang
    // disubmit (misal user langsung ketik tanpa nunggu observer manapun)
    // tetap membawa riwayat yang baru saja di-load.
    syncHistoryField(history);

    if (thread) {
      // SSE PatchElements mode "before" menyisipkan node baru sbg sibling
      // langsung #jena-pending — childList (bukan subtree) cukup krn
      // JenaAITurn (gomponents g.Group) merender flat, tanpa wrapper.
      var observer = new MutationObserver(function (mutations) {
        thread.scrollTop = thread.scrollHeight;

        for (var i = 0; i < mutations.length; i++) {
          var added = mutations[i].addedNodes;
          for (var j = 0; j < added.length; j++) {
            var node = added[j];
            if (
              node.nodeType === 1 &&
              node.classList &&
              node.classList.contains("jena-turn-data")
            ) {
              var turn = extractTurn(node);
              if (turn) {
                history.push(turn);
                if (history.length > JENA_HISTORY_MAX) {
                  history = history.slice(history.length - JENA_HISTORY_MAX);
                }
                saveHistory(history);
                syncHistoryField(history);
              }
            }
          }
        }
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

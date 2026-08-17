// Inisialisasi chart ECharts untuk panel aktivitas. File terpisah (bukan inline)
// agar lolos CSP script-src 'self'. echarts.min.js (UMD) sudah dimuat sebelum ini,
// mengekspos global `echarts`.
//
// Data & seluruh option chart ditanam server sebagai <script type="application/json">
// (JSON TIDAK dieksekusi browser → CSP-safe; go-echarts yang meng-inline <script>
// eksekusi DILARANG di sini). Kita JSON.parse textContent lalu setOption.
(function () {
  "use strict";
  if (typeof echarts === "undefined") return;

  function ready(fn) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", fn);
    } else {
      fn();
    }
  }

  // Tema ECharts mengikuti tema daisyUI (data-theme di <html>). daisyUI punya
  // banyak tema; sederhanakan jadi "dark" vs default berdasar color-scheme yang
  // di-set tiap tema (root punya `color-scheme`). Fallback: cek nama tema gelap.
  function isDarkTheme() {
    var t = document.documentElement.getAttribute("data-theme") || "";
    if (t) return /dark|dim|sunset|night|black|halloween|forest|luxury|dracula|business|coffee/.test(t);
    return window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
  }

  // Render satu chart: el = kontainer, dataEl = <script type=application/json>.
  // Mengembalikan instance agar bisa di-resize.
  function renderChart(elId, dataId, dark) {
    var el = document.getElementById(elId);
    var raw = document.getElementById(dataId);
    if (!el || !raw) return null;
    var opt;
    try {
      opt = JSON.parse(raw.textContent); // CSP-safe: data, bukan kode
    } catch (e) {
      return null; // JSON rusak — jangan gagalkan halaman
    }
    var chart = echarts.init(el, dark ? "dark" : null, { renderer: "canvas" });
    // Transparankan latar agar menyatu dgn card daisyUI (tema dark ECharts default gelap solid).
    opt.backgroundColor = "transparent";
    chart.setOption(opt);
    return chart;
  }

  // Temukan tiap kontainer chart lewat elemen data-nya (auto-discovery, bukan
  // daftar ID tertulis-tangan): tiap halaman yang menanam <script type=
  // application/json id="X-data"> + <div id="X"> otomatis ke-render tanpa
  // menyentuh file ini lagi (mis. Beranda M1 menambah chart-pipeline/-health
  // tanpa mengubah baris di bawah).
  function discoverChartIds() {
    var nodes = document.querySelectorAll('script[type="application/json"][id$="-data"]');
    var ids = [];
    for (var i = 0; i < nodes.length; i++) {
      var dataId = nodes[i].id;
      ids.push(dataId.slice(0, -5)); // buang akhiran "-data"
    }
    return ids;
  }

  ready(function () {
    var dark = isDarkTheme();
    var ids = discoverChartIds();
    var charts = [];
    for (var i = 0; i < ids.length; i++) {
      var c = renderChart(ids[i], ids[i] + "-data", dark);
      if (c) charts.push(c);
    }
    if (charts.length === 0) return;
    // Resize responsif (debounce sederhana).
    var t;
    window.addEventListener("resize", function () {
      clearTimeout(t);
      t = setTimeout(function () {
        for (var i = 0; i < charts.length; i++) charts[i].resize();
      }, 150);
    });
  });
})();

package ui

// jena_ai.go — widget Jena AI (BL-162 PoC): tombol floating kanan-bawah + panel
// chat docked kanan (satu sisi, gaya referensi Aegis). Reuse mekanisme slide
// (translate-x-full / ClassOn("!translate-x-0", …)) persis pola shellSidebar
// (appshell.go), arah dibalik (docked kanan → closed state translate POSITIF,
// bukan negatif seperti sidebar kiri) — bukan mekanisme baru. Panel docked
// penuh tinggi dari KANAN di semua breakpoint: mobile w-full, desktop (md:+)
// w-96 tetap — beda dari draft awal (kartu mengambang). Tombol trigger
// bottom-24 (BUKAN bottom-4, lihat komentar JenaAIWidget) — hindari tumpuk
// visual dengan toast yang sama-sama kanan-bawah (bottom-4 right-4).
//
// Non-streaming: balasan dikirim utuh sekali jadi lewat SSE PatchElements mode
// before ke #jena-pending, di dalam #jena-thread (internal/handler/jena_ai.go)
// — bukan token-per-token.

import (
	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// JenaAIWidget merender tombol + panel Jena AI. show=false → tak render apa
// pun (bukan CSS disembunyikan): postURL menunjuk endpoint bergerbang Casbin
// ai:chat/use, jadi user tanpa akses tak perlu melihatnya di markup sama
// sekali — pola sama quickLinks/notifBlock (nil/kosong → g.Text("")).
func JenaAIWidget(show bool, postURL string) g.Node {
	if !show {
		return g.Text("")
	}
	return g.Group([]g.Node{
		// Tombol trigger. Kanan-bawah (konsisten satu sisi dengan panel, gaya
		// Aegis) — TAPI bottom-24 (bukan bottom-4) supaya TIDAK presis di posisi
		// toast (bottom-4 right-4, components.go): toast transient (fade 3.2s,
		// pointer-events:none di wrapper luar jadi tak memblokir klik) tapi tetap
		// bisa menimpa visual tombol kalau posisinya sama persis — offset
		// vertikal cukup, lihat TestJenaAIWidget_PositionNoCollision. Disembunyikan
		// saat drawer sidebar mobile terbuka (sama alasan floating hamburger
		// appshell.go) agar tak tumpuk backdrop.
		h.Button(
			h.Type("button"),
			h.Class("btn btn-circle btn-primary fixed bottom-24 right-4 z-40 shadow-lg"),
			g.Attr("aria-label", "Buka Jena AI"),
			data.Show("!$sidebarOpen"),
			data.On("click", "$jenaOpen = !$jenaOpen"),
			lucide.MessageCircle(h.Class("size-6")),
		),

		jenaPanel(postURL),
	})
}

// jenaPanel = panel chat. Docked penuh tinggi dari KANAN di KEDUA breakpoint
// (gaya referensi Aegis: flush ke tepi layar, tanpa sudut membulat, persis
// posisi mockup) — beda dari draft awal (kartu kecil mengambang di desktop),
// diganti setelah user konfirmasi ingin persis gaya referensi lalu diminta
// docked kanan (bukan kiri seperti implementasi awal). Closed state
// translate-x-full (POSITIF — geser ke arah kanan, keluar viewport), beda
// dari shellSidebar yang -translate-x-full (negatif, sisi kiri).
//
// ClassOn pakai "!translate-x-0" (penting, BUKAN typo/kelebihan tanda seru):
// Tailwind v4 mengurutkan utility DI CSS OUTPUT menurut kategori internalnya,
// BUKAN urutan di source Go — `.translate-x-full` (positif) ternyata terbit
// SETELAH `.translate-x-0` di app.css, jadi tanpa `!` dua class itu bentrok
// spesifisitas SAMA dan yang belakangan di stylesheet menang → panel permanen
// off-screen walau $jenaOpen true (bug nyata, ditemukan lewat inspeksi
// app.css). Arah KIRI (shellSidebar, -translate-x-full) kebetulan lolos sebab
// urutan terbalik (negatif < translate-x-0 di output) — bukan berarti pola itu
// aman digeneralisasi ke arah lain tanpa `!`. Mobile = w-full, desktop
// (md:+) = lebar tetap w-96 — satu-satunya beda antar breakpoint;
// posisi/tinggi (inset-y-0 right-0, flush, tanpa rounded) SAMA di keduanya.
func jenaPanel(postURL string) g.Node {
	return h.Div(
		h.ID("jena-panel"),
		h.Class("fixed inset-y-0 right-0 z-40 flex w-full min-w-0 flex-col "+
			"border-l border-base-300 bg-base-100 text-base-content shadow-xl "+
			"translate-x-full transition-transform duration-200 md:w-96"),
		ClassOn("!translate-x-0", "$jenaOpen"),

		jenaHeader(),
		h.Div(
			h.ID("jena-thread"),
			h.Class("flex-1 min-h-0 overflow-y-auto p-3 space-y-3"),
			jenaWelcomeBubble(),
			// jenaPendingBubble SELALU dirender terakhir (display:none via
			// data-show saat idle) — jangkar tetap untuk SSE PatchElements
			// (WithSelectorID("jena-pending") + WithModeBefore, jena_ai.go
			// handler): bubble jawaban permanen selalu disisipkan TEPAT SEBELUM
			// elemen ini, jadi urutan kronologis terjaga walau anchor-nya sendiri
			// tak pernah pindah posisi (selalu paling akhir di DOM).
			jenaPendingBubble(),
		),
		jenaForm(postURL),
	)
}

func jenaHeader() g.Node {
	return h.Div(
		h.Class("flex items-center justify-between gap-2 border-b border-base-300 p-3"),
		h.Div(
			h.Class("flex items-center gap-2 min-w-0"),
			lucide.Bot(h.Class("size-5 text-primary shrink-0")),
			h.Span(h.Class("font-semibold truncate"), g.Text("Jena AI")),
		),
		h.Button(
			h.Type("button"),
			h.Class("btn btn-ghost btn-xs btn-circle"),
			g.Attr("aria-label", "Tutup Jena AI"),
			data.On("click", "$jenaOpen = false"),
			lucide.X(h.Class("size-4")),
		),
	)
}

// jenaWelcomeBubble = satu bubble AI statis di awal — bukan hasil panggilan
// proxy (hemat biaya untuk sapaan tetap), style sama dengan bubble balasan.
func jenaWelcomeBubble() g.Node {
	return h.Div(
		h.Class("chat chat-start jena-bubble-in"),
		h.Div(h.Class("chat-bubble"),
			g.Text("Halo, saya Jena AI. Tanya apa saja seputar fitur & alur kerja CRM ini.")),
	)
}

// jenaPendingBubble = bubble OPTIMISTIC: pertanyaan (dari signal $jenaPending,
// disalin form sebelum di-reset — lihat jenaForm) + tiga titik loading,
// tampil SELAMA $jenaLoading true (data-indicator otomatis oleh @post ask).
// Ini yang membuat submit terasa langsung merespons ("pertanyaan > send >
// muncul di panel > loading jawaban") alih-alih diam sampai jawaban lengkap.
// data.Text (bukan g.Text) sengaja: kontennya reaktif ke signal, bukan string
// server-side, tapi tetap AMAN — Datastar mem-set textContent (bukan innerHTML)
// jadi tak ada risiko injeksi walau isi signal berasal dari input user.
func jenaPendingBubble() g.Node {
	return h.Div(
		h.ID("jena-pending"),
		h.Class("space-y-3"),
		data.Show("$jenaLoading"),
		h.Div(
			h.Class("chat chat-end jena-bubble-in"),
			h.Div(h.Class("chat-bubble chat-bubble-primary"), data.Text("$jenaPending")),
		),
		h.Div(
			h.Class("chat chat-start jena-bubble-in"),
			h.Div(h.Class("chat-bubble"),
				g.Attr("aria-label", "Jena AI sedang mengetik"),
				h.Span(h.Class("loading loading-dots loading-sm")),
			),
		),
	)
}

// jenaForm — form native (bukan FormPostSelect, itu khusus <select>). Datastar
// auto-preventDefault() event submit saat data-on-submit menempel LANGSUNG di
// <form> (dikonfirmasi dari static/datastar.js), jadi TIDAK ada navigasi native
// walau tanpa modifier .prevent. contentType:'form' butuh <form> terdekat
// (gotcha #6) — dipenuhi struktural di sini.
//
// Urutan TIGA pernyataan submit SENGAJA: (1) salin nilai input ke $jenaPending
// SEBELUM apa pun lain — inilah teks yang tampil di bubble optimistic;
// (2) panggil @post — expression Datastar dikompilasi jadi Function SINKRON
// biasa (dikonfirmasi dari static/datastar.js: `Function("el","$","__action",
// "evt",...)`, BUKAN AsyncFunction), tapi action-nya sendiri `async` dan
// membaca FormData (contentType:'form') di bagian SINKRON sebelum await
// fetch pertamanya — jadi walau (3) evt.target.reset() menyusul di baris yang
// sama, form SUDAH terbaca lebih dulu saat @post dipanggil; reset TIDAK
// menghapus nilai yang sedang dikirim, hanya mengosongkan input di layar
// supaya user tak merasa "nyangkut" di teks lama. data.Indicator("jenaLoading")
// men-toggle signal otomatis: true saat @post berangkat, false lagi begitu
// selesai (sukses ATAUPUN gagal) — dipakai jenaPendingBubble & tombol kirim.
func jenaForm(postURL string) g.Node {
	return h.FormEl(
		h.Class("flex items-center gap-2 border-t border-base-300 p-3"),
		data.On("submit", "$jenaPending = evt.target.message.value; "+
			"@post('"+postURL+"', {contentType:'form'}); evt.target.reset()"),
		data.Indicator("jenaLoading"),
		h.Input(
			h.Type("text"),
			h.Name("message"),
			h.Class("input input-bordered input-sm text-base flex-1 min-w-0"),
			h.Placeholder("Tanya Jena AI..."),
			h.Required(),
			h.MaxLength("2000"),
			g.Attr("autocomplete", "off"),
		),
		h.Button(
			h.Type("submit"),
			h.Class("btn btn-primary btn-sm btn-circle"),
			// btn-disabled SAAT loading: cegah kirim pertanyaan kedua menimpa
			// $jenaPending yang sedang ditampilkan sebelum jawaban pertama tiba.
			ClassOn("btn-disabled", "$jenaLoading"),
			g.Attr("aria-label", "Kirim"),
			lucide.Send(h.Class("size-4")),
		),
	)
}

// JenaAIMessagePair merender satu pasang bubble (pertanyaan user + jawaban AI)
// — fragment yang disisipkan SEBELUM #jena-pending via SSE (jena_ai.go
// handler), menggantikan tampilan optimistic dengan bubble permanen. g.Text
// meng-escape isi (gotcha #15): pertanyaan & jawaban tak pernah dipercaya
// sebagai markup — termasuk jawaban AI, walau guardrail (claudeai/client.go)
// sudah minta teks polos tanpa markdown. whitespace-pre-line HANYA di bubble
// jawaban: baris baru ASLI dari model (\n, dikirim apa adanya lewat g.Text)
// dirender sebagai baris terpisah, bukan digabung jadi satu paragraf — tanpa
// ini `white-space: normal` bawaan browser meruntuhkan semua \n jadi spasi.
// Pertanyaan user tak butuh ini (input single-line, mustahil mengandung \n).
func JenaAIMessagePair(question, answer string) g.Node {
	return g.Group([]g.Node{
		h.Div(
			h.Class("chat chat-end jena-bubble-in"),
			h.Div(h.Class("chat-bubble chat-bubble-primary"), g.Text(question)),
		),
		h.Div(
			h.Class("chat chat-start jena-bubble-in"),
			h.Div(h.Class("chat-bubble whitespace-pre-line"), g.Text(answer)),
		),
	})
}

package ui

import (
	"strconv"
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// pager.go — pager keyset dua arah lewat "jejak cursor" di URL (BL-7).
//
// Cursor keyset itu forward-only: sebuah `?after=<nano>_<id>` hanya tahu "baris
// sesudah posisi ini", jadi ia tak bisa menghitung mundur ke halaman sebelumnya
// tanpa query terbalik per-list (churn SQL besar) atau OFFSET/COUNT (menyalahi
// keyset). Solusi tanpa keduanya: bawa JEJAK cursor halaman-halaman yang sudah
// dilewati di query string. "Sebelumnya" = pop jejak; "Berikutnya" = push cursor
// halaman ini. Nomor "Hal N" (halaman SAAT INI) turun gratis dari panjang jejak —
// bukan total halaman (itu butuh COUNT; sengaja di luar cakupan, lihat BL-7 b).
//
// Konsekuensi yang diterima sadar: URL memanjang seiring kedalaman, dan halaman
// yang di-bookmark dalam (tanpa `trail`) tampil sebagai Hal 1 tanpa tombol
// Sebelumnya — maju tetap jalan. Ini selaras sifat cursor yang memang tak
// ditandatangani (lihat internal/handler/pagecursor.go): jejak yang diarang
// pengguna paling banter memindah target "Sebelumnya" di daftar yang SAMA, yang
// izinnya sudah dijaga; token rusak diperlakukan sebagai jejak kosong.

const (
	// pagerTrailSep memisahkan entri jejak. Tilde: URL-unreserved (tanpa escaping)
	// dan mustahil muncul di cursor (cursor cuma digit + underscore).
	pagerTrailSep = "~"
	// pagerFirstToken menandai "after kosong" (halaman pertama) di dalam jejak.
	// Strip tunggal: juga mustahil jadi cursor, jadi tak rancu dengan cursor asli.
	pagerFirstToken = "-"
	// pagerTrailMax membatasi panjang jejak dari URL agar tak jadi vektor DoS
	// (jejak sepanjang ribuan entri). Di atas ini → diperlakukan jejak kosong.
	pagerTrailMax = 4096
)

// KeysetPager merender baris navigasi [« Sebelumnya] [Hal N] [Berikutnya »].
//
// baseHref = URL kanonik daftar yang SUDAH memuat semua param filter (view/tab/
// status/q) TANPA `after`/`trail`; helper menambahkan `after`/`trail` sendiri
// dengan pemisah yang benar. next = NextCursor daftar ("" = halaman terakhir).
//
// Link native <a> (navigasi = alamat berubah → bookmarkable, dibuka tab baru,
// reload; sekaligus lolos gotcha #16 sse.Redirect diblokir CSP). flex-wrap +
// min-h-11 (tap target ≥44px) mengikuti pola AppShell mobile-first.
func KeysetPager(baseHref, after, trail, next string) g.Node {
	entries := splitTrail(trail)
	hasPrev := len(entries) > 0
	hasNext := next != ""

	if !hasPrev && !hasNext {
		// Halaman tunggal MURNI (tanpa cursor sama sekali): tak ada apa pun untuk
		// dinavigasi — jangan render kontrol. nil aman: gomponents melewati anak
		// nil, jadi pemanggil boleh menyisipkannya tanpa syarat.
		if after == "" {
			return nil
		}
		// Deep-link ke halaman akhir TANPA jejak (mis. bookmark ?after=…): maju &
		// mundur sama-sama mustahil tanpa jejak. Nyatakan ujung saja — tanpa nomor
		// "Hal N" palsu (jejak tak ada, jadi nomornya tak diketahui) — supaya tak
		// tampak sama dengan halaman yang tombolnya gagal dirender.
		return h.Div(h.Class("flex flex-wrap items-center gap-2"),
			h.Span(h.Class("text-sm text-base-content/60"), g.Text("Ujung daftar.")))
	}

	children := make([]g.Node, 0, 3)

	if hasPrev {
		last := entries[len(entries)-1]
		prevAfter := ""
		if last != pagerFirstToken {
			prevAfter = last
		}
		prevTrail := joinTrail(entries[:len(entries)-1])
		children = append(children, h.A(
			h.Href(pagerHref(baseHref, prevAfter, prevTrail)),
			h.Class("btn btn-ghost min-h-11"),
			g.Text("« Sebelumnya"),
		))
	}

	// pageNum = halaman SAAT INI: jejak menyimpan halaman 1..K-1, jadi K = len+1.
	children = append(children, h.Span(
		h.Class("text-sm text-base-content/70"),
		g.Text("Hal "+strconv.Itoa(len(entries)+1)),
	))

	if hasNext {
		// Token halaman ini didorong ke jejak agar halaman berikut bisa mundur
		// ke sini. after kosong (hal 1) disimpan sebagai sentinel.
		cur := pagerFirstToken
		if after != "" {
			cur = after
		}
		nextTrail := joinTrail(append(append([]string{}, entries...), cur))
		children = append(children, h.A(
			h.Href(pagerHref(baseHref, next, nextTrail)),
			h.Class("btn min-h-11"),
			g.Text("Berikutnya »"),
		))
	} else {
		children = append(children, h.Span(
			h.Class("text-sm text-base-content/60"),
			g.Text("Ujung daftar."),
		))
	}

	return h.Div(h.Class("flex flex-wrap items-center gap-2"), g.Group(children))
}

// splitTrail memecah nilai `?trail=` jadi entri, membuang yang jelas rusak.
// Lenient by design (lihat catatan di atas): jejak kosong/kelewat panjang/berisi
// token tak-valid → nil (halaman diperlakukan sebagai tanpa jejak).
func splitTrail(trail string) []string {
	if trail == "" || len(trail) > pagerTrailMax {
		return nil
	}
	parts := strings.Split(trail, pagerTrailSep)
	for _, p := range parts {
		if !validTrailToken(p) {
			return nil
		}
	}
	return parts
}

// joinTrail merakit kembali entri jadi satu nilai `?trail=`.
func joinTrail(entries []string) string {
	return strings.Join(entries, pagerTrailSep)
}

// validTrailToken menerima sentinel halaman-pertama atau bentuk cursor
// (`<digit..>_<digit..>`). Divalidasi lokal (tanpa impor handler) agar paket ui
// tak bergantung ke handler; cukup memastikan token tak menyuntik karakter aneh
// ke URL.
func validTrailToken(s string) bool {
	if s == pagerFirstToken {
		return true
	}
	at, id, ok := strings.Cut(s, "_")
	return ok && isDigits(at) && isDigits(id)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// pagerHref menambahkan param after/trail ke baseHref. Nilai after/trail hanya
// berisi digit, `_`, `-`, `~` (semua URL-unreserved) jadi tak perlu di-escape;
// baseHref sudah membawa param filternya yang ter-escape dari pemanggil.
func pagerHref(baseHref, after, trail string) string {
	href := baseHref
	if after != "" {
		href = withParam(href, "after", after)
	}
	if trail != "" {
		href = withParam(href, "trail", trail)
	}
	return href
}

func withParam(href, key, val string) string {
	sep := "?"
	if strings.Contains(href, "?") {
		sep = "&"
	}
	return href + sep + key + "=" + val
}

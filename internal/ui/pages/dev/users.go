// Package dev berisi halaman panel developer (/dev).
package dev

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// UserRow = data satu baris user untuk tabel (view model, bukan db.User).
type UserRow struct {
	ID    int64
	Email string
	// Name = nama tampilan dari provider; "" bila tak diketahui (akun password
	// dev, atau Google tak mengirimkannya). Di panel platform email TETAP tampil
	// utuh di samping nama — operator platform memakainya untuk mencocokkan
	// orang, dan nama tidak unik.
	Name          string
	Status        string
	AvatarURL     string
	IsRoot        bool            // super-admin env (kontrol dinonaktifkan)
	Workspaces    []WorkspaceRole // keanggotaan: role PER-workspace (model membership)
	Quota         int             // kuota EFEKTIF (override bila ada, selainnya default global)
	QuotaOverride bool            // true = angka di atas hak KHUSUS user ini, bukan warisan global
}

// WorkspaceRole = satu keanggotaan user (workspace + role di dalamnya). Role kini
// per-workspace, jadi panel platform menampilkan/mengubah per baris ini.
type WorkspaceRole struct {
	TenantID int64
	Name     string
	Role     string
}

// UsersView = data siap-render halaman /dev/users.
//
// Dikumpulkan jadi satu struct alih-alih daftar parameter yang terus memanjang:
// dengan lima nilai bertipe mirip (dua bool bersebelahan), pemanggil yang
// tertukar urutannya tetap lolos compiler — dan salahnya baru terlihat sebagai
// tombol yang muncul di tempat yang keliru.
type UsersView struct {
	Rows           []UserRow
	Roles          []string // role yang boleh diberikan (mode single tak mengenal owner, 0006 §7)
	CanManageSuper bool     // aktor boleh mengangkat super-admin (opsi super_admin muncul)
	// NextCursor = penanda halaman berikutnya; "" berarti ini halaman terakhir.
	NextCursor string
	// HasPrev = halaman ini BUKAN yang pertama. Cursor keyset tak bisa mundur
	// (ia hanya tahu "sesudah X"), jadi jalan kembalinya ke awal daftar — dan
	// itu harus dikatakan apa adanya lewat label, bukan disamarkan sebagai
	// "Sebelumnya" yang melompat ke tempat tak terduga.
	HasPrev bool
}

// UsersPage merender tabel user + kontrol role/status/hapus.
func UsersPage(v UsersView) g.Node {
	return h.Div(
		h.H1(h.Class("text-xl font-semibold mb-4"), g.Text("Users")),
		// Slot toast (kanan-bawah), diisi via SSE patch (id "flash").
		// pointer-events:none agar toast tak memblokir klik elemen di bawahnya.
		h.Div(h.ID("flash"), h.Class("fixed bottom-4 right-4 z-50"),
			g.Attr("style", "pointer-events:none")),
		h.Div(
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(
				h.Class("card-body min-w-0"),
				ui.TableScroll(h.Table(
					h.Class("w-full text-sm"),
					h.THead(
						h.Tr(
							h.Class("border-b border-base-300 text-left text-base-content/70"),
							th("User"), th("Role"), th("Kuota"), th("Status"), th("Aksi"),
						),
					),
					h.TBody(g.Map(v.Rows, func(u UserRow) g.Node {
						return UserRowNode(u, v.Roles, v.CanManageSuper)
					})),
				)),
				usersPager(v),
			),
		),
	)
}

// usersPager = jalan ke halaman berikutnya (dan kembali ke awal).
//
// Link biasa, bukan Datastar: berpindah halaman adalah NAVIGASI — alamatnya
// berubah, jadi ia harus bisa di-bookmark, dibuka di tab baru, dan dimuat ulang.
// Ini juga menghindari gotcha #16 (sse.Redirect diblokir CSP) tanpa perlu
// menyentuhnya sama sekali.
//
// Tak dirender saat hanya ada satu halaman: kontrol navigasi yang tak menuju ke
// mana pun cuma mengundang klik yang tak berbuat apa-apa.
func usersPager(v UsersView) g.Node {
	if v.NextCursor == "" && !v.HasPrev {
		return nil
	}
	// flex-wrap di BARIS TOMBOL: dua tombol + keterangan tak muat di 375px dan
	// akan mendorong lebar halaman bila tak boleh membungkus (mobile-first).
	//
	// Ukuran penuh (`btn`, BUKAN `btn-sm`): ini navigasi utama halaman, dan
	// `btn-sm` hanya 32px — di bawah ambang tap target 44px. Kontrol sekunder di
	// dalam baris tabel boleh kecil; yang memindahkan halaman tidak.
	return h.Div(
		h.Class("mt-4 flex flex-wrap items-center gap-2"),
		g.If(v.HasPrev, h.A(
			h.Href("/dev/users"), h.Class("btn btn-ghost min-h-11"),
			g.Text("« Awal daftar"),
		)),
		g.If(v.NextCursor != "", h.A(
			h.Href("/dev/users?after="+v.NextCursor), h.Class("btn min-h-11"),
			g.Text("Berikutnya »"),
		)),
		// Ujung daftar dikatakan eksplisit. Tanpa ini, halaman terakhir tampak
		// sama dengan halaman yang tombolnya gagal dirender — dan operator akan
		// mengira masih ada user yang tak bisa ia jangkau.
		g.If(v.NextCursor == "", h.Span(
			h.Class("text-sm text-base-content/60"),
			g.Text("Ujung daftar."),
		)),
	)
}

// UserRowNode merender satu baris user. Di-export agar handler bisa me-render
// ulang baris ini via SSE setelah mutasi (id "user-<id>" jadi target morph).
func UserRowNode(u UserRow, roles []string, canManageSuper bool) g.Node {
	rowID := "user-" + strconv.FormatInt(u.ID, 10)
	return h.Tr(
		h.ID(rowID),
		h.Class("border-b border-base-300"),
		// Kolom user: avatar + nama/email (+ badge root).
		h.Td(
			h.Class("py-2 pr-4"),
			h.Div(
				h.Class("flex items-center gap-2 min-w-0"),
				ui.Avatar(u.AvatarURL, u.Name, u.Email, 32),
				userIdent(u),
				ui.When(u.IsRoot, badge("root", "outline")),
			),
		),
		h.Td(h.Class("py-2 pr-4"), roleControl(u, roles, canManageSuper)),
		h.Td(h.Class("py-2 pr-4"), quotaControl(u)),
		h.Td(h.Class("py-2 pr-4"), statusControl(u)),
		h.Td(h.Class("py-2"), deleteControl(u)),
	)
}

// userIdent = nama di atas, email di bawahnya. Berbeda dari panel workspace,
// email di sini SELALU utuh: yang membacanya adalah operator platform, yang
// justru bertugas mencocokkan orang — dan nama tak bisa dipakai untuk itu
// (tidak unik, dan nilainya dikendalikan usernya sendiri di Google).
//
// Tanpa nama, email naik jadi baris utama alih-alih menyisakan baris kosong.
func userIdent(u UserRow) g.Node {
	if u.Name == "" {
		return h.Span(h.Class("truncate"), g.Text(u.Email))
	}
	return h.Div(
		h.Class("min-w-0"),
		h.Div(h.Class("truncate"), g.Text(u.Name)),
		h.Div(h.Class("truncate text-xs text-base-content/60"), g.Text(u.Email)),
	)
}

// Flash merender toast notifikasi (id "flash", target SSE patch). Auto-hilang
// via animasi CSS (.toast-flash, fade-out) — TANPA inline script (patuh CSP
// script-src 'self'). ok=true → hijau (alert-success), false → merah (alert-error).
// Catatan: class fade kustom bernama .toast-flash (BUKAN .toast) — daisyUI punya
// .toast sendiri (kontainer posisi) yang akan bentrok.
func Flash(ok bool, msg string) g.Node {
	cls := "alert alert-success toast-flash shadow-lg"
	if !ok {
		cls = "alert alert-error toast-flash shadow-lg"
	}
	return h.Div(
		h.ID("flash"),
		h.Class("fixed bottom-4 right-4 z-50"),
		g.Attr("style", "pointer-events:none"), // toast tak boleh blokir klik
		h.Div(
			h.Class(cls),
			h.Role("status"),
			g.Text(msg),
		),
	)
}

func th(label string) g.Node {
	return h.Th(h.Class("py-2 pr-4 font-medium"), g.Text(label))
}

// badge merender lencana daisyUI. variant "outline" → badge-outline; selain itu
// (kosong) → badge-neutral (abu solid) untuk role/status immutable root.
func badge(text, variant string) g.Node {
	cls := "badge badge-neutral"
	if variant == "outline" {
		cls = "badge badge-outline"
	}
	return h.Span(h.Class(cls), g.Text(text))
}

package dev

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// settings.go — pengaturan seluruh platform. Wadah umum, bukan halaman
// "kuota": pengaturan berikutnya (registrasi buka/tutup, TTL undangan, masa
// tenggang purge) punya rumah di sini tanpa menambah menu lagi.

// SettingsView = data siap-render halaman pengaturan platform.
type SettingsView struct {
	QuotaDefault int    // jatah workspace untuk user yang BELUM di-override
	QuotaMin     int    // batas bawah yang diterima backend
	QuotaMax     int    // batas atas
	OverrideN    int    // berapa user memegang hak khusus — konteks dampak
	Msg          string // pesan sukses
	Err          string // pesan galat

	// Retensi jejak audit, dalam hari.
	RetentionDays int
	RetentionMin  int
	RetentionMax  int

	// SingleMode = aplikasi masih berjalan sebagai satu aplikasi. Setelah naik ke
	// multi ini permanen false — form kenaikan diganti keterangan keadaan, sebab
	// tombol yang tak punya efek lebih buruk daripada tombol yang tak ada.
	SingleMode bool
	// PrimaryName = nama workspace primer, ditampilkan agar operator tahu PERSIS
	// apa yang akan tetap ada setelah kenaikan (alamatnya pun tak berubah).
	PrimaryName string
}

// Settings merender form pengaturan platform.
func Settings(v SettingsView) g.Node {
	body := []g.Node{
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Pengaturan Platform")),
		h.P(h.Class("text-base-content/70 mb-4"),
			g.Text("Aturan yang berlaku untuk seluruh platform. Perubahan langsung aktif tanpa restart.")),
	}
	if v.Err != "" {
		body = append(body, ui.Toast(ui.VariantDestructive, "settings-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Toast(ui.VariantSuccess, "settings-msg", g.Text(v.Msg)))
	}
	body = append(body, tenancyCard(v), quotaCard(v), retentionCard(v))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// retentionCard = berapa lama jejak audit disimpan sebelum dibuang otomatis.
//
// Disajikan dengan menyebut APA yang hilang, bukan cuma "masa simpan": angka di
// sini menentukan pertanyaan mana yang masih bisa dijawab tahun depan, dan
// menurunkannya adalah penghapusan permanen yang tak diberi tahu siapa pun saat
// benar-benar terjadi (pembersihannya berjalan diam-diam di latar).
func retentionCard(v SettingsView) g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-1"), g.Text("Masa Simpan Jejak Aktivitas")),
			h.P(h.Class("text-sm text-base-content/70 mb-3"),
				g.Text("Jejak yang lebih tua dari batas ini dihapus permanen oleh "+
					"pemeliharaan harian. Menaikkannya menyimpan lebih banyak; "+
					"menurunkannya membuang yang sudah lewat batas baru — dan itu "+
					"tak bisa dibatalkan.")),
			h.FormEl(
				h.Method("post"), h.Action("/dev/settings/retention"),
				h.Class("flex flex-wrap items-end gap-3 min-w-0"),
				h.Div(
					h.Class("grid gap-2"),
					ui.Label("Hari", h.For("retention-days")),
					ui.Input(
						h.ID("retention-days"), h.Name("days"), h.Type("number"),
						h.Value(strconv.Itoa(v.RetentionDays)),
						g.Attr("min", strconv.Itoa(v.RetentionMin)),
						g.Attr("max", strconv.Itoa(v.RetentionMax)),
						h.Required(),
						h.Class("input w-32"),
					),
				),
				h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text("Simpan")),
			),
			h.P(h.Class("text-xs text-base-content/60 mt-2"),
				g.Text("Minimal "+strconv.Itoa(v.RetentionMin)+" hari — sama dengan masa "+
					"tenggang workspace terhapus, agar jejak penghapusan tak hilang "+
					"sebelum workspace-nya sendiri benar-benar dibuang.")),
		),
	)
}

// quotaCard = kuota workspace default. Form POST native → 303 (gotcha #16).
func quotaCard(v SettingsView) g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-1"), g.Text("Kuota Workspace")),
			h.P(h.Class("text-sm text-base-content/70 mb-3"),
				g.Text("Berapa workspace yang boleh DIMILIKI seorang user. "+
					"Diundang sebagai anggota workspace orang lain tidak memakan kuota.")),
			h.FormEl(
				h.Method("post"), h.Action("/dev/settings/quota"),
				h.Class("flex flex-wrap items-end gap-3 min-w-0"),
				h.Div(
					h.Class("grid gap-2"),
					ui.Label("Default global", h.For("quota")),
					ui.Input(
						h.ID("quota"), h.Name("quota"), h.Type("number"),
						h.Value(strconv.Itoa(v.QuotaDefault)),
						g.Attr("min", strconv.Itoa(v.QuotaMin)),
						g.Attr("max", strconv.Itoa(v.QuotaMax)),
						h.Required(),
						h.Class("input w-32"),
					),
				),
				h.Button(h.Type("submit"), h.Class("btn btn-primary"), g.Text("Simpan")),
			),
			// Dampak perubahan dinyatakan eksplisit: tanpa ini operator tak tahu
			// bahwa user ber-hak-khusus SENGAJA tak ikut berubah, lalu mengira
			// pengaturannya tak bekerja.
			h.P(h.Class("text-xs text-base-content/60 mt-3"),
				g.Text("Berlaku bagi semua user yang belum diberi hak khusus. "+
					overrideNote(v.OverrideN)+
					" Hak khusus per-user diatur di halaman Users.")),
		),
	)
}

// overrideNote menyusun kalimat jumlah pengecualian. Nol pengecualian ≠ kalimat
// kosong: "0 user" tetap informasi yang menenangkan saat operator ragu.
func overrideNote(n int) string {
	if n == 0 {
		return "Saat ini tidak ada user dengan hak khusus."
	}
	return strconv.Itoa(n) + " user memegang hak khusus dan TIDAK ikut berubah."
}

// tenancyCard = kenaikan mode tenancy: satu aplikasi → multi-workspace.
//
// SEKALI JALAN, dan itu ditegakkan DATABASE (trigger menolak penurunan), bukan
// oleh ketiadaan tombol di sini. Karena itu kartunya harus jujur soal
// permanennya: aksi ireversibel yang disajikan seperti toggle biasa adalah
// jebakan, bukan fitur.
//
// Konfirmasi lewat mengetik nama workspace primer — bukan checkbox. Checkbox
// bisa dicentang tanpa dibaca; menyalin nama menuntut orangnya melihat objek
// yang terlibat.
func tenancyCard(v SettingsView) g.Node {
	if !v.SingleMode {
		// Sudah multi. Tak ada form — hanya keadaan, supaya tak ada yang mencari
		// tombol turun yang memang tak akan pernah ada.
		return h.Div(
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(
				h.Class("card-body min-w-0"),
				h.H2(h.Class("font-semibold mb-1"), g.Text("Mode Tenancy")),
				h.P(h.Class("text-sm text-base-content/70"),
					g.Text("Aplikasi ini berjalan sebagai MULTI-WORKSPACE. Setiap user "+
						"dapat memiliki workspace sendiri, dibatasi kuota di bawah.")),
				h.P(h.Class("text-xs text-base-content/60 mt-2"),
					g.Text("Mode ini permanen — turun kembali ke satu aplikasi akan "+
						"menyembunyikan setiap workspace selain yang utama, jadi database menolaknya.")),
			),
		)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-warning/40 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-1"), g.Text("Mode Tenancy")),
			h.P(h.Class("text-sm text-base-content/70 mb-3"),
				g.Text("Aplikasi ini berjalan sebagai SATU APLIKASI. Semua orang bekerja "+
					"di ruang yang sama, dan tak ada yang bisa membuat workspace sendiri.")),
			// Yang PALING sering ditanyakan sebelum menekan: apa yang berubah, dan
			// apakah tautan lama tetap hidup. Dijawab sebelum ditanya.
			h.Ul(
				h.Class("text-sm list-disc pl-5 mb-3 space-y-1 text-base-content/80"),
				h.Li(g.Text("Setiap user dapat membuat workspace sendiri (dibatasi kuota).")),
				h.Li(g.Text("Menu ganti workspace muncul, dan alamat "+
					"ruang kerja saat ini TIDAK berubah — tak ada tautan yang mati.")),
				h.Li(g.Text("Berlaku seketika, tanpa restart.")),
				h.Li(h.Class("text-warning"),
					g.Text("PERMANEN: tidak ada jalan kembali ke satu aplikasi.")),
			),
			h.FormEl(
				h.Method("post"), h.Action("/dev/settings/tenancy"),
				h.Class("flex flex-wrap items-end gap-3 min-w-0"),
				h.Div(
					h.Class("grid gap-2 min-w-0"),
					ui.Label("Ketik nama aplikasi untuk mengonfirmasi: "+v.PrimaryName, h.For("confirm")),
					ui.Input(
						h.ID("confirm"), h.Name("confirm"), h.Type("text"),
						h.Required(), h.AutoComplete("off"),
						h.Placeholder(v.PrimaryName),
						h.Class("input"),
					),
				),
				h.Button(h.Type("submit"), h.Class("btn btn-warning"),
					g.Text("Naikkan ke multi-workspace")),
			),
		),
	)
}

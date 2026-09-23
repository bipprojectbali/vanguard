package panel

import (
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// role_edit_hints.go — keterangan kecil "belum terjangkau UI" di bawah nama
// modul pada matriks peran (moduleHint dkk), dipisah dari role_edit.go
// (sudah di ambang batas 500 baris/20.000 karakter — konvensi sama
// role_edit_levels.go) agar penambahan modul bergerbang lain ke depannya tak
// mendorongnya lewat batas.

// moduleHints = keterangan kecil di bawah nama modul untuk baris yang izinnya
// bisa tersimpan sah tapi TAK TERJANGKAU UI tanpa Active Subscriptions
// ("crm:subscriptions") ≥ Lihat juga (audit 2026-09, lihat catatan
// reachability di business_defaults.go dekat entri crmModules
// "crm:renewals"/"crm:churn"). Renewals & Churn: SubscriptionDetail/
// SubscriptionRenewals/SubscriptionChurnList (tempat tombol Renew/Churn
// dirender) semua digerbangi canViewSubscriptions ("crm:subscriptions" read),
// BUKAN crm:renewals/crm:churn — jadi "Lihat"/"Kelola" di baris ini percuma
// selama Active Subscriptions="Tak ada". Sengaja DIDOKUMENTASIKAN (bukan
// diubah gate-nya) — lihat riwayat commit branch
// chore/document-renewals-churn-prereq utk diskusi keputusannya.
//
// KONDISIONAL (permintaan user, bukan lagi selalu tampil seperti versi
// pertama commit 89cd1bb): keterangan HANYA muncul saat kombinasi
// bermasalah NYATA terjadi (baris ini ≥ Lihat DAN Active
// Subscriptions=Tak ada) — supaya peran yang sudah benar (mis. Manager
// bawaan, Active Subscriptions="Kelola") tak menampilkan peringatan yang tak
// relevan baginya. Lihat moduleHint untuk cara kondisinya dihitung di dua
// mode (canEdit reaktif vs read-only statis).
var moduleHints = map[string]string{
	"crm:renewals": "Butuh Active Subscriptions ≥ Lihat juga, agar halamannya terjangkau.",
	"crm:churn":    "Butuh Active Subscriptions ≥ Lihat juga, agar halamannya terjangkau.",
}

// moduleHint merender <p> kecil di bawah label modul bila m.Obj punya entri
// di moduleHints DAN kombinasi levelnya saat ini bermasalah, atau g.Text("")
// (node kosong, aman digabung g.Group) bila tidak — dipisah dari
// roleMatrixRow agar map lookup+kondisi tak mengotori badan fungsi yang
// sudah padat reaktivitas Datastar.
//
// canEdit=true → dipasang data.Show mengacu signal LIVE ($lvl_<sufiks baris
// ini> & $lvl_subscriptions, KEDUANYA sudah dideklarasikan data.Signals oleh
// baris masing-masing krn canEdit=true, dan baris Active Subscriptions
// SELALU mendahului Renewals/Churn di crmModules/urutan dokumen — syarat
// "dideklarasikan lebih dulu di dokumen" di doc roleMatrixRow terpenuhi),
// sehingga hint muncul/hilang SAAT ITU JUGA ketika admin mengubah salah satu
// dropdown, tanpa reload. subsLevel (nilai saat render) TAK dipakai di jalur
// ini — cuma penanda awal, Datastar mengambil alih sepenuhnya setelah
// hidrasi.
//
// canEdit=false (peninjau read-only, mis. workspace arsip) → baris ini tak
// pernah dapat data.Signals (lihat roleMatrixRow), jadi tak ada signal utk
// dirujuk; dihitung STATIS dari m.Level & subsLevel (level Active
// Subscriptions TERSIMPAN, dioper roleMatrix) sekali saat render.
func moduleHint(m RoleModulePerm, canEdit bool, subsLevel string) g.Node {
	hint, ok := moduleHints[m.Obj]
	if !ok {
		return g.Text("")
	}
	if !canEdit {
		if m.Level == "none" || subsLevel != "none" {
			return g.Text("")
		}
		return h.P(h.Class("text-xs text-warning font-normal"), g.Text(hint))
	}
	expr := "$lvl_" + moduleSignal(m.Obj) + "!=='none'&&$lvl_subscriptions==='none'"
	return h.P(h.Class("text-xs text-warning font-normal"), data.Show(expr), g.Text(hint))
}

// moduleLevelOf mencari level modul obj dalam mods (dipakai roleMatrix agar
// moduleHint tahu level Active Subscriptions saat merender baris
// Renewals/Churn). Tak ketemu (semestinya tak pernah terjadi — crmModules
// statis & selalu memuat crm:subscriptions) → "none", pilihan paling ketat/
// aman: hint condong TAMPIL, bukan diam-diam disembunyikan oleh kegagalan
// pencarian.
func moduleLevelOf(mods []RoleModulePerm, obj string) string {
	for _, m := range mods {
		if m.Obj == obj {
			return m.Level
		}
	}
	return "none"
}

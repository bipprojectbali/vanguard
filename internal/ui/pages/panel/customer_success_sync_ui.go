package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// customer_success_sync_ui.go — komponen UI khusus sinkronisasi Product
// Adoption dari Desa+ (BL-27), dipisah dari customer_success_view.go agar
// tiap file tetap di bawah ambang tipe View/Component (300 baris).

// customerSuccessSyncForm = tombol "Sinkron dari Desa+" (BL-27). Form NATIVE
// POST → 303 (gotcha #16: hasil sync memuat ulang SELURUH halaman lewat
// redirect PRG ?ok=/?err=, bukan fragment parsial — jadi native POST, BUKAN
// Datastar @post; sama pola deleteAccountForm di accounts_detail.go).
func customerSuccessSyncForm(action string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(action),
		h.Button(h.Type("submit"), h.Class("btn btn-sm btn-outline min-h-11"),
			g.Text("Sinkron dari Desa+")),
	)
}

// telemetrySourceNote menjelaskan cakupan PARSIAL sync (BL-27): hanya 3 dari 6
// field Adoption/Usage benar-benar berasal dari desa-plus (Aktivitas Terakhir/
// Pengguna Aktif/Fitur Utama Dipakai) — sisanya (Frekuensi Login/Tingkat Adopsi
// Fitur/Tren Penggunaan) tetap isian manual walau badge "Sumber Data" berbunyi
// "Product Telemetry". Teks singkat di bawah kartu (pola sama WriteLockNote),
// BUKAN tooltip — supaya CSM tak salah kira seluruh section otomatis.
func telemetrySourceNote() g.Node {
	return h.P(h.Class("text-xs text-base-content/60 -mt-2"),
		g.Text("Sumber Data \"Product Telemetry\" hanya mencakup Aktivitas Terakhir, "+
			"Pengguna Aktif, dan Fitur Utama Dipakai — Frekuensi Login, Tingkat Adopsi "+
			"Fitur, dan Tren Penggunaan tetap isian manual."))
}

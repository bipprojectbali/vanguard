package handler

import (
	"net/http"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// workspace_codes.go — pengaturan FORMAT kode-unik entitas per workspace
// (DESA-001, DEAL-042, …). Halaman tersendiri di /w/{slug}/codes:
// enam entitas × satu form membuat halaman pengaturan utama terlalu panjang bila
// digabung, jadi ia dipisah — tapi tetap di bawah gerbang yang SAMA
// (canEditWorkspace), bukan gerbang baru yang bisa bergeser sendiri.
//
// Nomornya sendiri dialokasikan atomik di titik pembuatan entitas
// (db.GenerateEntityCode); di sini HANYA bentuknya yang diatur. Mengubah format
// tak menyentuh kode yang sudah terbit (kode disimpan di baris entitas, tak
// dihitung ulang).

// WorkspaceCodeFormats — GET /w/{workspace}/codes. Menampilkan format
// tiap entitas: yang tersimpan bila ada, default bawaan bila belum. canEdit
// mengikuti canEditWorkspace — pengelola boleh ubah, selain itu hanya lihat.
func (h *Handler) WorkspaceCodeFormats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Format tersimpan → map by entity, agar entitas yang belum diset jatuh ke
	// default tanpa satu query per entitas.
	rows, err := h.q(ctx).ListCodeFormats(ctx, session.TenantID(ctx))
	if err != nil {
		h.Log.Error("codes: list formats", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	saved := make(map[string]db.CodeFormat, len(rows))
	for _, row := range rows {
		saved[row.Entity] = row
	}

	// Urutan tampil = urutan AllEntities (stabil), bukan urutan baris DB.
	// EntityAccount (Desa) sengaja TAK dirender: desa kini beridentitas kode
	// wilayah Kemendagri, bukan format DESA-xxx internal, jadi konfiguratornya
	// tak relevan. Kode account masih dialokasikan di backend (accounts_codes.go)
	// — ini hanya menyembunyikan kartu konfigurasi, bukan menghentikan alokasi.
	items := make([]panel.CodeFormatItem, 0, len(codes.AllEntities()))
	for _, e := range codes.AllEntities() {
		if e == codes.EntityAccount {
			continue
		}
		f := codes.DefaultFormat(e)
		isDefault := true
		if row, ok := saved[string(e)]; ok {
			f = codes.Format{Prefix: row.Prefix, Separator: row.Separator, Padding: int(row.Padding)}
			isDefault = false
		}
		items = append(items, panel.CodeFormatItem{
			Entity:    string(e),
			Label:     entityLabel(e),
			Prefix:    f.Prefix,
			Separator: f.Separator,
			Padding:   f.Padding,
			// Contoh bentuk (nomor 1) — ilustrasi format, bukan alokasi nyata.
			Preview:   f.Render(1),
			IsDefault: isDefault,
		})
	}

	h.renderWorkspaceShell(w, r, "Format Kode", "/codes", panel.CodeFormats(panel.CodeFormatsView{
		Base:         wsPath(slugFromRequest(r), ""),
		CanEdit:      canEditWorkspace(ctx),
		Items:        items,
		MaxPrefix:    codes.MaxPrefixLen,
		MaxSeparator: codes.MaxSeparatorLen,
		MinPadding:   codes.MinPadding,
		MaxPadding:   codes.MaxPadding,
		Msg:          codeFormatMsg(r.URL.Query().Get("ok")),
		Err:          wsErrMsg(r.URL.Query().Get("err")),
	}))
}

// entityLabel = nama tampilan entitas dalam Bahasa Indonesia. Peta eksplisit,
// bukan Title-case otomatis: "account" harus tampil "Desa (Akun)" sesuai domain
// (Desa = pelanggan HUB), dan istilah CRM lain sudah lazim dipakai apa adanya.
// Entitas tak dikenal (mustahil setelah codes.Valid, tapi jaga-jaga) → nilai
// mentahnya, bukan string kosong.
func entityLabel(e codes.Entity) string {
	switch e {
	case codes.EntityAccount:
		return "Desa (Akun)"
	case codes.EntityLead:
		return "Prospek (Lead)"
	case codes.EntityDeal:
		return "Kesepakatan (Deal)"
	case codes.EntityQuote:
		return "Penawaran (Quote)"
	case codes.EntityTicket:
		return "Tiket"
	case codes.EntitySubscription:
		return "Langganan"
	default:
		return string(e)
	}
}

// codeFormatMsg memetakan kode sukses (`?ok=`) → pesan. Dipisah dari wsErrMsg
// (yang khusus galat) supaya alert sukses & galat tak pernah tertukar variannya.
func codeFormatMsg(code string) string {
	if code == "saved" {
		return "Format kode disimpan. Kode yang sudah terbit tidak berubah."
	}
	return ""
}

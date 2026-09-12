package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5"
)

// sales_convert_action_input.go — validasi & resolusi PRA-tx untuk LeadConvert
// (sales_convert_action.go): guard tulis + parse form, lalu resolusi desa &
// cek dup. Keduanya SEBELUM tx atomik dimulai — memisahkannya TIDAK memecah
// tx tunggal LeadConvert (yang sengaja dibiarkan utuh).

// resolveConvertRequest memvalidasi & mem-parse permintaan LeadConvert: guard
// tulis, target lead ber-scope kepemilikan, status Qualified & belum converted
// (convert_guard), lalu parse form. F4: aktor tanpa akses nomor penuh diselamatkan
// dari mengirim mask — nomor asli lead disalin server-side, bukan mask. HP &
// WhatsApp digabung jadi satu field UI (mobile_phone) — form konversi tak lagi
// menyertakan WhatsApp; nomor WA lead disalin apa adanya ke kontak baru.
func (h *Handler) resolveConvertRequest(w http.ResponseWriter, r *http.Request) (db.Lead, convertForm, int64, bool) {
	if !h.requireLeadWrite(w, r) {
		return db.Lead{}, convertForm{}, 0, false
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return db.Lead{}, convertForm{}, 0, false
	}
	l, ok := h.loadOwnedLead(w, r, id)
	if !ok {
		return db.Lead{}, convertForm{}, 0, false
	}
	idStr := strconv.FormatInt(id, 10)
	if l.LeadStatus != "Qualified" || l.Converted {
		wsRedirect(w, r, "/leads/"+idStr, "convert_guard")
		return db.Lead{}, convertForm{}, 0, false
	}

	form, errCode := parseConvertForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/leads/"+idStr+"/convert", errCode)
		return db.Lead{}, convertForm{}, 0, false
	}
	if !canEditPhone(ctx) {
		form.MobilePhone = l.MobilePhone
	}
	form.Whatsapp = l.Whatsapp
	return l, form, id, true
}

// resolveConvertVillage menyelesaikan desa hasil konversi dari master Kemendagri
// (regions level 4) — SAMA seperti AccountCreate langsung — agar akun hasil
// convert punya village_code Kemendagri asli & tunduk aturan "satu desa hidup =
// satu akun" (BL-67). WAJIB: konversi hanya saat lead Qualified (convert_guard)
// → villageID sudah pasti; nil → village_required. Desa tak dikenal (id palsu /
// bukan level 4) → "village_id". Blokir (bukan soft-warning nama) bila desa ini
// sudah punya akun HIDUP di workspace → tolak konversi & tautkan operator ke akun
// eksisting (dup=<id>, dirender jadi <a> di halaman review). Pre-check lewat
// SELECT SEBELUM INSERT karena konversi = satu tx atomik — pelanggaran UNIQUE
// (idx_accounts_code) akan meracuni seluruh tx (pola sama VillageCodeExists).
// Unique index tetap jaring balapan (accountWriteErr, di LeadConvert).
func (h *Handler) resolveConvertVillage(w http.ResponseWriter, r *http.Request, villageID *int64, tenantID int64, convertErr string) (db.GetVillageRegionRow, bool) {
	ctx := r.Context()
	if villageID == nil {
		wsRedirect(w, r, convertErr, "village_required")
		return db.GetVillageRegionRow{}, false
	}
	reg, err := h.q(ctx).GetVillageRegion(ctx, *villageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			wsRedirect(w, r, convertErr, "village_id")
			return db.GetVillageRegionRow{}, false
		}
		h.Log.Error("convert: get village region", "err", err)
		wsRedirect(w, r, convertErr, "failed")
		return db.GetVillageRegionRow{}, false
	}
	vcode := reg.Code

	if existing, derr := h.q(ctx).GetAccountByVillageCode(ctx, db.GetAccountByVillageCodeParams{
		TenantID:    tenantID,
		VillageCode: &vcode,
	}); derr == nil {
		wsRedirect(w, r, convertErr, "village_code_dup&dup="+strconv.FormatInt(existing.ID, 10))
		return db.GetVillageRegionRow{}, false
	} else if !errors.Is(derr, pgx.ErrNoRows) {
		h.Log.Error("convert: check village dup", "err", derr)
		wsRedirect(w, r, convertErr, "failed")
		return db.GetVillageRegionRow{}, false
	}
	return reg, true
}

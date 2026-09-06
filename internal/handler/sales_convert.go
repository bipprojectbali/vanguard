package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_convert.go — HALAMAN review pra-isi konversi Lead → Desa+Kontak+Deal &
// convertPrefill. Form (parseConvertForm) ada di sales_convert_form.go; aksi
// atomik (LeadConvert) ada di sales_convert_action.go — dipisah dari sini semata
// untuk file health karena ia menyentuh TIGA tabel lain + menautkan balik ke
// lead, tumbuh dengan aturannya sendiri.

// LeadConvertPage — GET /w/{workspace}/leads/{id}/convert. Halaman review
// pra-isi dari lead. Gerbang: tulis leads (F2) + kepemilikan (F3, loadOwnedLead)
// + status Qualified & belum converted. Di luar itu → redirect dengan sebab.
func (h *Handler) LeadConvertPage(w http.ResponseWriter, r *http.Request) {
	if !h.requireLeadWrite(w, r) {
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	l, ok := h.loadOwnedLead(w, r, id)
	if !ok {
		return
	}
	idStr := strconv.FormatInt(id, 10)
	if l.LeadStatus != "Qualified" || l.Converted {
		wsRedirect(w, r, "/leads/"+idStr, "convert_guard")
		return
	}

	ctx := r.Context()
	base := wsPath(slugFromRequest(r), "")
	phoneEditable := canEditPhone(ctx) // F4: hanya Sales lihat/sunting nomor penuh.
	errCode := r.URL.Query().Get("err")
	v := panel.LeadConvertView{
		Base:          base,
		Action:        base + "/leads/" + idStr + "/convert",
		BackURL:       base + "/leads/" + idStr,
		Err:           wsErrMsg(errCode),
		LeadName:      l.LeadName,
		LeadCode:      deref(l.EntityCode),
		PhoneEditable: phoneEditable,
		RegionsJSON:   h.regionsJSON(ctx),
		VillagesURL:   base + "/accounts/villages", // BL-67: lazy-fetch Desa level 4
		AccountTypes:  accountTypeOptions,
		Fields:        convertPrefill(l, phoneEditable),
		Duplicates:    h.findDuplicateVillages(ctx, session.TenantID(ctx), l.LeadName, l.DistrictID),
	}
	// BL-67: blokir desa yang sudah ber-akun (village_code_dup) menautkan operator
	// ke akun eksisting. Handler POST menaruh id-nya di ?dup=; muat nama + kode
	// sistem untuk label tautan. Soft-fail: akun tak termuat → cukup pesan tanpa
	// tautan (Err tetap terisi).
	if errCode == "village_code_dup" {
		if dupID, code := optInt64(r.URL.Query().Get("dup")); code == "" && dupID != nil {
			if a, err := h.q(ctx).GetAccount(ctx, *dupID); err == nil {
				v.DupAccountID = a.ID
				v.DupAccountLabel = a.VillageName
				if ec := deref(a.EntityCode); ec != "" {
					v.DupAccountLabel += " (" + ec + ")"
				}
			}
		}
	}
	h.renderWorkspaceShell(w, r, "Konversi Lead", "/leads", panel.LeadConvert(v))
}

// convertPrefill memetakan Lead → nilai review pra-isi. Nomor telepon disamarkan
// bila aktor bukan Sales (F4): nilai asli TAK pernah dikirim ke browsernya —
// handler POST menyalin nomor asli lead server-side sebagai gantinya.
func convertPrefill(l db.Lead, phoneEditable bool) panel.ConvertFormFields {
	firstName := deref(l.ContactPerson)
	if firstName == "" {
		firstName = l.LeadName // tak ada nama kontak → pakai nama lead sebagai awal.
	}
	mobile, whatsapp := deref(l.MobilePhone), deref(l.Whatsapp)
	if !phoneEditable {
		if mobile != "" {
			mobile = flsHidden
		}
		if whatsapp != "" {
			whatsapp = flsHidden
		}
	}
	return panel.ConvertFormFields{
		AccountType: "prospect", // lead yang dikonversi = prospek baru.
		// BL-67: Desa kini dipilih dari master Kemendagri saat convert (bukan
		// diwarisi nama lead). Kecamatan lead diprefill sbg filter; dropdown Desa
		// mulai kosong (lead tak menyimpan village_id).
		DistrictID:  int64PtrStr(l.DistrictID),
		FirstName:   firstName,
		JobTitle:    deref(l.JobTitle),
		MobilePhone: mobile,
		Whatsapp:    whatsapp,
		Email:       deref(l.Email),
		DealName:    l.LeadName,
		Amount:      numericStr(l.EstimatedValue),
	}
}

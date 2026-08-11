package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_convert.go — INTI Modul 4: konversi Lead Qualified → Desa (Account) +
// Kontak (Contact) + Deal dalam SATU tx ber-tenant yang atomik. Bukan satu-klik:
// GET menampilkan halaman review PRA-ISI dari lead yang bisa disunting, POST
// menjalankan konversi. Dipisah dari sales_leads.go karena ia menyentuh TIGA
// tabel lain + menautkan balik ke lead — tumbuh dengan aturannya sendiri.
//
// Atomicity: tx Scope-middleware SELALU commit (run mengembalikan nil), TAPI
// Postgres membatalkan seluruh tx pada galat query APA PUN → Commit jadi
// rollback. Maka INSERT account→contact→deal + UPDATE lead lewat h.q(ctx) yang
// sama = all-or-nothing tanpa savepoint: bila CreateContact gagal setelah
// CreateAccount sukses, account ikut ter-rollback. Pada galat pertama: log +
// wsRedirect ?err= + return; tx auto-rollback. (Mekanisme sama seperti "nomor
// tak terbakar" di AccountCreate.)
//
// Guard: hanya lead Qualified & belum converted boleh dikonversi (dicek SEBELUM
// tx). MarkLeadConverted juga menjaga di WHERE (idempotent-aman), tapi guard
// awal memberi pesan yang bisa dipahami alih-alih diam.

// convertForm = nilai review konversi yang SUDAH divalidasi. Menggabungkan tiga
// entitas hasil (desa + kontak utama + deal) menjadi satu submit.
type convertForm struct {
	// Desa (Account)
	VillageName string
	AccountType string
	Province    *string
	Regency     *string
	District    *string
	// Kontak utama (Contact)
	FirstName   string
	LastName    *string
	JobTitle    *string
	MobilePhone *string
	Whatsapp    *string
	Email       *string
	// Deal
	DealName string
	Amount   pgtype.Numeric
}

// parseConvertForm membaca & memvalidasi form review. (form, "") bila sah, atau
// (zero, kode) yang dipetakan wsErrMsg. Enum & panjang dicermin dari form desa/
// deal (sumber sama: validAccountTypes, maxVillageNameLen, maxDealNameLen,
// maxContactNameLen) agar konversi tak menerima nilai yang ditolak create biasa.
func parseConvertForm(fv func(string) string) (convertForm, string) {
	var f convertForm

	f.VillageName = strings.TrimSpace(fv("village_name"))
	if f.VillageName == "" || len(f.VillageName) > maxVillageNameLen {
		return convertForm{}, "village_name"
	}
	f.AccountType = strings.TrimSpace(fv("account_type"))
	if _, ok := validAccountTypes[f.AccountType]; !ok {
		return convertForm{}, "account_type"
	}

	f.FirstName = strings.TrimSpace(fv("first_name"))
	if f.FirstName == "" || len(f.FirstName) > maxContactNameLen {
		return convertForm{}, "first_name"
	}

	f.DealName = strings.TrimSpace(fv("deal_name"))
	if f.DealName == "" || len(f.DealName) > maxDealNameLen {
		return convertForm{}, "deal_name"
	}
	amt, code := optNumeric(fv("amount"), "amount")
	if code != "" {
		return convertForm{}, code
	}
	f.Amount = amt

	// Teks bebas opsional: trim, kosong → NULL.
	f.Province = optTrim(fv("province"))
	f.Regency = optTrim(fv("regency"))
	f.District = optTrim(fv("district"))
	f.LastName = optTrim(fv("last_name"))
	f.JobTitle = optTrim(fv("job_title"))
	f.MobilePhone = optTrim(fv("mobile_phone"))
	f.Whatsapp = optTrim(fv("whatsapp_number"))
	f.Email = optTrim(fv("email"))

	return f, ""
}

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
	v := panel.LeadConvertView{
		Base:          base,
		Action:        base + "/leads/" + idStr + "/convert",
		BackURL:       base + "/leads/" + idStr,
		Err:           wsErrMsg(r.URL.Query().Get("err")),
		LeadName:      l.LeadName,
		LeadCode:      deref(l.EntityCode),
		PhoneEditable: phoneEditable,
		AccountTypes:  accountTypeOptions,
		Fields:        convertPrefill(l, phoneEditable),
	}
	h.renderWorkspaceShell(w, r, "Konversi Lead", "/leads", panel.LeadConvert(v))
}

// LeadConvert — POST /w/{workspace}/leads/{id}/convert. Menjalankan konversi
// dalam satu tx atomik. Sukses → /deals/{newID}?ok=converted.
func (h *Handler) LeadConvert(w http.ResponseWriter, r *http.Request) {
	if !h.requireLeadWrite(w, r) {
		return
	}
	ctx := r.Context()
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

	form, errCode := parseConvertForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/leads/"+idStr+"/convert", errCode)
		return
	}
	// F4: aktor tak berhak lihat nomor penuh mengirim mask (field dikunci) — mask
	// TAK boleh jadi nomor kontak baru. Salin nomor asli lead server-side; nilai
	// tersamar tak pernah kembali ke DB.
	if !canEditPhone(ctx) {
		form.MobilePhone = l.MobilePhone
		form.Whatsapp = l.Whatsapp
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)
	// Pemilik entitas hasil = pemilik lead (pertahankan rep penjualan agar manajer
	// yang mengonversi tak diam-diam mengambil alih); fallback aktor bila lead tak
	// berpemilik. CreatedBy = aktor (siapa yang menekan tombol).
	owner := l.LeadOwner
	if owner == nil {
		owner = &uid
	}
	convertErr := "/leads/" + idStr + "/convert"

	// --- SATU tx ber-tenant (h.q ambient): gagal-sebagian = rollback penuh. ---
	accountCode, err := h.q(ctx).GenerateEntityCode(ctx, tenantID, codes.EntityAccount)
	if err != nil {
		h.Log.Error("convert: account code", "err", err)
		wsRedirect(w, r, convertErr, "failed")
		return
	}
	acc, err := h.q(ctx).CreateAccount(ctx, db.CreateAccountParams{
		TenantID:     tenantID,
		EntityCode:   &accountCode,
		VillageName:  form.VillageName,
		AccountType:  form.AccountType,
		AccountOwner: owner,
		Province:     form.Province,
		Regency:      form.Regency,
		District:     form.District,
		CreatedBy:    &uid,
	})
	if err != nil {
		if code, ok := accountWriteErr(err); ok {
			wsRedirect(w, r, convertErr, code)
			return
		}
		h.Log.Error("convert: account", "err", err)
		wsRedirect(w, r, convertErr, "failed")
		return
	}

	contact, err := h.q(ctx).CreateContact(ctx, db.CreateContactParams{
		TenantID:         tenantID,
		AccountID:        acc.ID,
		ContactOwner:     owner,
		FirstName:        form.FirstName,
		LastName:         form.LastName,
		JobTitle:         form.JobTitle,
		IsPrimaryContact: true, // kontak hasil konversi = kontak utama desa baru.
		MobilePhone:      form.MobilePhone,
		WhatsappNumber:   form.Whatsapp,
		Email:            form.Email,
		CreatedBy:        &uid,
	})
	if err != nil {
		h.Log.Error("convert: contact", "err", err)
		wsRedirect(w, r, convertErr, "failed")
		return
	}

	dealCode, err := h.q(ctx).GenerateEntityCode(ctx, tenantID, codes.EntityDeal)
	if err != nil {
		h.Log.Error("convert: deal code", "err", err)
		wsRedirect(w, r, convertErr, "failed")
		return
	}
	deal, err := h.q(ctx).CreateDeal(ctx, db.CreateDealParams{
		TenantID:         tenantID,
		EntityCode:       &dealCode,
		DealName:         form.DealName,
		AccountID:        acc.ID,
		DealOwner:        owner,
		PrimaryContactID: &contact.ID,
		Stage:            "Prospecting", // deal lahir di awal pipeline.
		Amount:           form.Amount,
		CreatedBy:        &uid,
	})
	if err != nil {
		h.Log.Error("convert: deal", "err", err)
		wsRedirect(w, r, convertErr, "failed")
		return
	}

	if err := h.q(ctx).MarkLeadConverted(ctx, db.MarkLeadConvertedParams{
		ConvertedAccountID: &acc.ID,
		ConvertedContactID: &contact.ID,
		ConvertedDealID:    &deal.ID,
		UpdatedBy:          &uid,
		ID:                 id,
	}); err != nil {
		h.Log.Error("convert: mark", "err", err)
		wsRedirect(w, r, convertErr, "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "lead.convert", tenantID, map[string]string{
		"lead_id":    idStr,
		"account_id": strconv.FormatInt(acc.ID, 10),
		"contact_id": strconv.FormatInt(contact.ID, 10),
		"deal_id":    strconv.FormatInt(deal.ID, 10),
	})
	wsRedirectOK(w, r, "/deals/"+strconv.FormatInt(deal.ID, 10), "converted")
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
		VillageName: l.LeadName,
		AccountType: "prospect", // lead yang dikonversi = prospek baru.
		Province:    deref(l.Province),
		Regency:     deref(l.Regency),
		District:    deref(l.District),
		FirstName:   firstName,
		JobTitle:    deref(l.JobTitle),
		MobilePhone: mobile,
		Whatsapp:    whatsapp,
		Email:       deref(l.Email),
		DealName:    l.LeadName,
		Amount:      numericStr(l.EstimatedValue),
	}
}

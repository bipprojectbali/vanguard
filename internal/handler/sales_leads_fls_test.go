package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// sales_leads_fls_test.go — F4 (field-level) nomor HP/WhatsApp di form sunting
// Lead: utuh HANYA Sales (canEditPhone), role lain menerima mask & tak bisa
// menimpa nilai tersimpan dengan mask itu. Diperbaiki audit FLS M9-1 (gap
// LIVE: Manager punya crm:leads write tapi di luar allow-list canSeeFullPhone,
// dan LeadEdit sebelumnya mengirim nomor asli mentah ke form). Pola sama
// dengan accounts_fls_test.go (F4_PhoneMasking / F4_NonSalesTakBisaTimpaPhone).

// seedLeadWithPhone menaruh satu Lead Qualified dgn nomor HP/WhatsApp — untuk
// menguji F4 tanpa merangkai create lewat handler.
func (e *testEnv) seedLeadWithPhone(t *testing.T, name string, owner int64, mobile, whatsapp string) db.Lead {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityLead)
	if err != nil {
		t.Fatalf("generate lead code: %v", err)
	}
	l, err := e.q.CreateLead(t.Context(), db.CreateLeadParams{
		TenantID:    e.tenantID,
		EntityCode:  &code,
		LeadName:    name,
		LeadOwner:   &owner,
		LeadStatus:  "Qualified",
		MobilePhone: &mobile,
		Whatsapp:    &whatsapp,
		CreatedBy:   &owner,
	})
	if err != nil {
		t.Fatalf("seed lead %s: %v", name, err)
	}
	return l
}

// leadFormValues merakit form values minimal sah untuk LeadUpdate.
func leadFormValues(name, status string) url.Values {
	return url.Values{
		"lead_name":   {name},
		"lead_status": {status},
	}
}

// TestLeadEdit_PhoneMasked: nomor HP/WhatsApp utuh HANYA Sales di form sunting;
// role lain yang tetap punya crm:leads write (Manager, Admin via crm:*) menerima
// mask. Nilai asli tak boleh SAMPAI ke browser non-Sales (view-source). CSM tak
// diuji di sini — CSM tak punya crm:leads write sama sekali (business_policy.csv),
// jadi ditolak F2 sebelum sempat menyentuh F4 (di luar cakupan test masking ini).
func TestLeadEdit_PhoneMasked(t *testing.T) {
	env, uid := setupAccounts(t)
	mobile, whatsapp := "0812-1111-2222", "0812-3333-4444"
	l := env.seedLeadWithPhone(t, "Lead Kontak", uid, mobile, whatsapp)

	cases := []struct {
		role string
		full bool
	}{
		{"sales", true},
		{"admin", false},
		{"manager", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/leads/"+itoa(l.ID)+"/edit", nil, itoa(l.ID))
			rec := env.runAccount(uid, "owner", c.role, req, env.h.LeadEdit)
			if rec.Code != http.StatusOK {
				t.Fatalf("LeadEdit status %d\n%s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			has := strings.Contains(body, mobile) || strings.Contains(body, whatsapp)
			if c.full && !has {
				t.Errorf("sales harus melihat nomor utuh di form")
			}
			if !c.full && has {
				t.Errorf("role %q BOCOR — nomor asli sampai ke browser (F4)", c.role)
			}
		})
	}
}

// TestLeadEdit_PhoneFieldsLocked (BL-84): form sunting untuk role tak-berhak-lihat
// (Manager) MENGUNCI field HP/WhatsApp — input disabled TANPA name, jadi mask "•••"
// tak akan ikut ter-submit. Sales tetap menerima field bernama & aktif. Ini akar
// perbaikan: dulu field bermask tetap punya name → browser mengirim "•••" → parse
// menolaknya → seluruh sunting gagal. Kunci simetris phoneField (accounts).
func TestLeadEdit_PhoneFieldsLocked(t *testing.T) {
	env, uid := setupAccounts(t)
	l := env.seedLeadWithPhone(t, "Lead Kontak", uid, "0812-1111-2222", "0812-3333-4444")

	cases := []struct {
		role       string
		wantSubmit bool // apakah field HP/WA punya name (ikut ter-submit)
	}{
		{"sales", true},
		{"manager", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/leads/"+itoa(l.ID)+"/edit", nil, itoa(l.ID))
			rec := env.runAccount(uid, "owner", c.role, req, env.h.LeadEdit)
			if rec.Code != http.StatusOK {
				t.Fatalf("LeadEdit status %d\n%s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			hasName := strings.Contains(body, `name="mobile_phone"`) || strings.Contains(body, `name="whatsapp"`)
			if c.wantSubmit && !hasName {
				t.Errorf("sales: field HP/WA harus ber-name (dapat disunting)")
			}
			if !c.wantSubmit {
				if hasName {
					t.Errorf("role %q: field HP/WA TAK boleh ber-name (mask akan ter-submit → bug BL-84)", c.role)
				}
				if !strings.Contains(body, "disabled") {
					t.Errorf("role %q: field HP/WA harus disabled (terkunci)", c.role)
				}
			}
		})
	}
}

// TestLeadUpdate_NonSalesBisaSimpan (BL-84 regresi): role tak-berhak-lihat (Manager)
// bisa MENYIMPAN sunting profil lead — field HP/WA terkunci di form tak ikut
// ter-submit (browser terperbaiki mengosongkannya), sunting field lain (contact_person)
// berhasil, dan nomor asli dipertahankan. Dulu bug: mask "•••" ter-submit → parse
// gagal → role itu mustahil menyimpan perubahan apa pun.
func TestLeadUpdate_NonSalesBisaSimpan(t *testing.T) {
	env, uid := setupAccounts(t)
	mobile, whatsapp := "0812-1111-2222", "0812-3333-4444"
	l := env.seedLeadWithPhone(t, "Lead Kontak", uid, mobile, whatsapp)

	// Browser terperbaiki: field HP/WA disabled → TAK dikirim. Ubah contact_person.
	form := leadFormValues("Lead Kontak", "Qualified")
	form.Set("contact_person", "Budi Baru")
	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(l.ID), form, itoa(l.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.LeadUpdate)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update gagal: %d\n%s", rec.Code, rec.Body.String())
	}
	// Redirect SUKSES (ke detail), BUKAN error redirect (ke /edit) — bug lama
	// mendarat di /edit?err=mobile_phone walau kode-nya juga 303.
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "/edit") {
		t.Fatalf("sunting non-Sales gagal validasi (redirect ke %q) — BL-84 belum terperbaiki", loc)
	}

	got, err := env.q.GetLead(t.Context(), l.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.ContactPerson == nil || *got.ContactPerson != "Budi Baru" {
		t.Errorf("contact_person harus tersimpan, got %v", got.ContactPerson)
	}
	if got.MobilePhone == nil || *got.MobilePhone != mobile {
		t.Errorf("mobile_phone asli harus dipertahankan, got %v", got.MobilePhone)
	}
	if got.Whatsapp == nil || *got.Whatsapp != whatsapp {
		t.Errorf("whatsapp asli harus dipertahankan, got %v", got.Whatsapp)
	}
}

// TestLeadUpdate_MaskPostTakMerusak (BL-84): POST usil/rusak yang tetap mengirim
// mask "•••" TAK boleh menimpa nomor asli. Parse menolak mask → error redirect,
// tapi tak ada baris tersentuh, jadi data tetap utuh (guard tulis tak tercapai,
// namun kegagalan parse melindungi via tak-mengubah-apa-apa).
func TestLeadUpdate_MaskPostTakMerusak(t *testing.T) {
	env, uid := setupAccounts(t)
	mobile, whatsapp := "0812-1111-2222", "0812-3333-4444"
	l := env.seedLeadWithPhone(t, "Lead Kontak", uid, mobile, whatsapp)

	form := leadFormValues("Lead Kontak", "Qualified")
	form.Set("mobile_phone", flsHidden)
	form.Set("whatsapp", flsHidden)
	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(l.ID), form, itoa(l.ID))
	if rec := env.runAccount(uid, "owner", "manager", req, env.h.LeadUpdate); rec.Code != http.StatusSeeOther {
		t.Fatalf("update gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	got, err := env.q.GetLead(t.Context(), l.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.MobilePhone == nil || *got.MobilePhone != mobile {
		t.Errorf("mobile_phone asli harus utuh, got %v (mask menimpa = F4 bocor)", got.MobilePhone)
	}
	if got.Whatsapp == nil || *got.Whatsapp != whatsapp {
		t.Errorf("whatsapp asli harus utuh, got %v (mask menimpa = F4 bocor)", got.Whatsapp)
	}
}

// TestLeadUpdate_PhoneEditable_Sales: Sales tetap bisa mengubah nomor HP/
// WhatsApp lewat form sunting (canEditPhone tak mengunci role yang berhak).
func TestLeadUpdate_PhoneEditable_Sales(t *testing.T) {
	env, uid := setupAccounts(t)
	l := env.seedLeadWithPhone(t, "Lead Kontak", uid, "0812-1111-2222", "0812-3333-4444")

	newMobile, newWhatsapp := "0899-9999-0000", "0899-8888-0000"
	form := leadFormValues("Lead Kontak", "Qualified")
	form.Set("mobile_phone", newMobile)
	form.Set("whatsapp", newWhatsapp)
	req := accountsReq(http.MethodPost, "/w/test/leads/"+itoa(l.ID), form, itoa(l.ID))
	if rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadUpdate); rec.Code != http.StatusSeeOther {
		t.Fatalf("update gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	got, err := env.q.GetLead(t.Context(), l.ID)
	if err != nil {
		t.Fatalf("get lead: %v", err)
	}
	if got.MobilePhone == nil || *got.MobilePhone != newMobile {
		t.Errorf("sales harus bisa mengubah mobile_phone, got %v", got.MobilePhone)
	}
	if got.Whatsapp == nil || *got.Whatsapp != newWhatsapp {
		t.Errorf("sales harus bisa mengubah whatsapp, got %v", got.Whatsapp)
	}
}

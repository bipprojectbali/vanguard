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

// TestLeadUpdate_PhoneNotOverwritten: editor non-Sales mengirim mask (field
// terkunci di view), tapi handler mempertahankan nomor ASLI — mask tak boleh
// menimpa nilai tersimpan (simetris AccountUpdate/ContactPhone).
func TestLeadUpdate_PhoneNotOverwritten(t *testing.T) {
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
		t.Errorf("mobile_phone asli harus dipertahankan, got %v (mask menimpa = F4 bocor)", got.MobilePhone)
	}
	if got.Whatsapp == nil || *got.Whatsapp != whatsapp {
		t.Errorf("whatsapp asli harus dipertahankan, got %v (mask menimpa = F4 bocor)", got.Whatsapp)
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

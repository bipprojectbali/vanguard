package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// sales_convert_phone_test.go — F4/FLS §5 pada halaman review konversi lead.
// Halaman ini dulu memakai SATU flag (canEditPhone, Sales-only) untuk memutuskan
// visibilitas SEKALIGUS editability nomor → Admin (yang berhak MELIHAT via
// canSeeFullPhone tapi tak berhak menyunting) ikut kena mask, tak konsisten dgn
// detail lead. Perbaikannya memisahkan dua sumbu. Yang dijaga di sini:
//
//   - Sales   : field bisa disunting (name="mobile_phone") berisi nomor asli.
//   - Admin   : nomor ASLI tampil, TAPI read-only (tanpa name, "Hanya-baca").
//   - Manager : nomor tersamar (•••), nomor asli TAK pernah sampai ke browser.
//
// Manager dipakai sbg kasus "tersamar" (bukan Support/CSM) karena ia satu-satunya
// peran yang PUNYA crm:leads write (lolos requireLeadWrite → sampai ke halaman
// ini) NAMUN di luar canSeeFullPhone (Sales+Admin saja). Support/CSM tak punya
// leads write → 403 sebelum halaman, tak relevan di sini.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler, pola sama
// sales_convert_test.go.

// seedQualifiedLeadPhone menaruh lead Qualified milik owner dgn HP/WhatsApp
// terisi — untuk menguji penyamaran nomor F4 di halaman konversi.
func (e *testEnv) seedQualifiedLeadPhone(t *testing.T, name string, owner int64, mobile, whatsapp string) db.Lead {
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

const (
	convPhoneMobile = "081234567890"
	convPhoneWA     = "089876543210"
)

// TestLeadConvertPage_Sales_PhoneEditable: Sales melihat nomor asli DI field yang
// bisa disunting (name="mobile_phone").
func TestLeadConvertPage_Sales_PhoneEditable(t *testing.T) {
	env, uid := setupAccounts(t)
	lead := env.seedQualifiedLeadPhone(t, "Lead HP Sales", uid, convPhoneMobile, convPhoneWA)

	req := accountsReq(http.MethodGet, "/w/test/leads/"+itoa(lead.ID)+"/convert", nil, itoa(lead.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadConvertPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("LeadConvertPage status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, convPhoneMobile) {
		t.Errorf("Sales harus melihat nomor asli HP, body:\n%s", body)
	}
	if !strings.Contains(body, `name="mobile_phone"`) {
		t.Errorf("Sales harus punya field HP yang bisa disunting (name=), body:\n%s", body)
	}
}

// TestLeadConvertPage_Admin_PhoneVisibleReadOnly: Admin (canSeeFullPhone tapi
// bukan canEditPhone) melihat nomor ASLI, namun field read-only — tanpa
// name="mobile_phone" dan disertai catatan "Hanya-baca". INI regresi yang
// diperbaiki: sebelumnya Admin kena mask (•••).
func TestLeadConvertPage_Admin_PhoneVisibleReadOnly(t *testing.T) {
	env, uid := setupAccounts(t)
	lead := env.seedQualifiedLeadPhone(t, "Lead HP Admin", uid, convPhoneMobile, convPhoneWA)

	req := accountsReq(http.MethodGet, "/w/test/leads/"+itoa(lead.ID)+"/convert", nil, itoa(lead.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.LeadConvertPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("LeadConvertPage status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, convPhoneMobile) {
		t.Errorf("Admin harus MELIHAT nomor asli HP/WhatsApp (FLS §5), body:\n%s", body)
	}
	if strings.Contains(body, flsHidden) {
		t.Errorf("Admin TAK boleh melihat mask %q — regresi F4, body:\n%s", flsHidden, body)
	}
	if strings.Contains(body, `name="mobile_phone"`) {
		t.Errorf("Admin: field HP harus read-only (tanpa name=), body:\n%s", body)
	}
	if !strings.Contains(body, "Hanya-baca") {
		t.Errorf("Admin: field HP harus bertanda read-only, body:\n%s", body)
	}
}

// TestLeadConvertPage_Manager_PhoneMasked: Manager (punya crm:leads write →
// sampai ke halaman, tapi bukan Sales maupun Admin) → nomor tersamar; nomor asli
// TAK pernah dioper ke view (kriteria F4).
func TestLeadConvertPage_Manager_PhoneMasked(t *testing.T) {
	env, uid := setupAccounts(t)
	lead := env.seedQualifiedLeadPhone(t, "Lead HP Manager", uid, convPhoneMobile, convPhoneWA)

	req := accountsReq(http.MethodGet, "/w/test/leads/"+itoa(lead.ID)+"/convert", nil, itoa(lead.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.LeadConvertPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("LeadConvertPage status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, convPhoneMobile) {
		t.Errorf("Manager TAK boleh menerima nomor asli (F4), body:\n%s", body)
	}
	if !strings.Contains(body, flsHidden) {
		t.Errorf("Manager harus melihat nomor tersamar %q, body:\n%s", flsHidden, body)
	}
	if !strings.Contains(body, "disamarkan") {
		t.Errorf("Manager: field HP harus bertanda tersamar, body:\n%s", body)
	}
}

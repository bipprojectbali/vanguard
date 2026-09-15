package handler

import (
	"net/http"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// sales_activities_contact_options_test.go — activityContactsForTarget (BL-164):
// resolusi opsi Kontak mengikuti Target dipilih, per tipe target + gate F3.

// resolveActivityContacts menjalankan activityContactsForTarget lewat runAccount
// (butuh ctx ber-Scope, sama pola test handler lain) — activityContactsForTarget
// sendiri bukan http.HandlerFunc, jadi dibungkus closure kecil di sini.
func (e *testEnv) resolveActivityContacts(t *testing.T, uid int64, wsRole, bizRole, targetType string, id int64) (
	[]panel.AccountMemberOption, string, *panel.LeadContactInfoView, error,
) {
	t.Helper()
	var contacts []panel.AccountMemberOption
	var preselect string
	var leadInfo *panel.LeadContactInfoView
	var resErr error
	req := accountsReq(http.MethodGet, "/activities/new", nil, "")
	e.runAccount(uid, wsRole, bizRole, req, func(w http.ResponseWriter, r *http.Request) {
		contacts, preselect, leadInfo, resErr = e.h.activityContactsForTarget(r.Context(), targetType, id)
	})
	return contacts, preselect, leadInfo, resErr
}

// TestActivityContactsForTarget_Account: account:<id> → kontak milik desa itu.
func TestActivityContactsForTarget_Account(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Kontak", &uid, nil, nil)
	c := env.seedContact(t, acc.ID, "Budi", &uid, false)

	contacts, preselect, leadInfo, err := env.resolveActivityContacts(t, uid, "member", "sales", "account", acc.ID)
	if err != nil {
		t.Fatalf("activityContactsForTarget: %v", err)
	}
	if leadInfo != nil {
		t.Errorf("account target tak boleh menghasilkan leadInfo")
	}
	if preselect != "" {
		t.Errorf("account target tak boleh auto-preselect, got %q", preselect)
	}
	if len(contacts) != 1 || contacts[0].ID != c.ID {
		t.Errorf("kontak harus berisi %d (Budi), got %+v", c.ID, contacts)
	}
}

// TestActivityContactsForTarget_Deal_PreselectsPrimaryContact: deal:<id> → kontak
// milik desa deal, DAN preselect = primary_contact_id bila terisi.
func TestActivityContactsForTarget_Deal_PreselectsPrimaryContact(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Deal", &uid, nil, nil)
	primary := env.seedContact(t, acc.ID, "Contact Primer", &uid, true)
	env.seedContact(t, acc.ID, "Contact Lain", &uid, false)

	code, err := env.q.GenerateEntityCode(t.Context(), env.tenantID, codes.EntityDeal)
	if err != nil {
		t.Fatalf("generate deal code: %v", err)
	}
	d, err := env.q.CreateDeal(t.Context(), db.CreateDealParams{
		TenantID:         env.tenantID,
		EntityCode:       &code,
		DealName:         "Deal Kontak",
		AccountID:        acc.ID,
		DealOwner:        &uid,
		PrimaryContactID: &primary.ID,
		Stage:            dealInitialStage,
		CreatedBy:        &uid,
	})
	if err != nil {
		t.Fatalf("seed deal: %v", err)
	}

	contacts, preselect, leadInfo, err := env.resolveActivityContacts(t, uid, "member", "sales", "deal", d.ID)
	if err != nil {
		t.Fatalf("activityContactsForTarget: %v", err)
	}
	if leadInfo != nil {
		t.Errorf("deal target tak boleh menghasilkan leadInfo")
	}
	if preselect != itoa(primary.ID) {
		t.Errorf("preselect harus %d (primary_contact_id), got %q", primary.ID, preselect)
	}
	if len(contacts) != 2 {
		t.Errorf("kontak harus berisi kedua kontak desa deal, got %+v", contacts)
	}
}

// TestActivityContactsForTarget_Contact_PreselectsSelf: contact:<id> → kontak
// difilter ke desa yang sama, DAN preselect = dirinya sendiri.
func TestActivityContactsForTarget_Contact_PreselectsSelf(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sendiri", &uid, nil, nil)
	c := env.seedContact(t, acc.ID, "Diri Sendiri", &uid, false)

	contacts, preselect, leadInfo, err := env.resolveActivityContacts(t, uid, "member", "sales", "contact", c.ID)
	if err != nil {
		t.Fatalf("activityContactsForTarget: %v", err)
	}
	if leadInfo != nil {
		t.Errorf("contact target tak boleh menghasilkan leadInfo")
	}
	if preselect != itoa(c.ID) {
		t.Errorf("preselect harus %d (dirinya sendiri), got %q", c.ID, preselect)
	}
	found := false
	for _, opt := range contacts {
		if opt.ID == c.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("kontak dirinya sendiri harus ada di opsi, got %+v", contacts)
	}
}

// TestActivityContactsForTarget_Lead_ReturnsInfoNoContacts: lead:<id> → TIDAK ada
// query kontak (Lead tak punya relasi ke contacts); leadInfo diisi dari field lead.
func TestActivityContactsForTarget_Lead_ReturnsInfoNoContacts(t *testing.T) {
	env, uid := setupAccounts(t)
	code, err := env.q.GenerateEntityCode(t.Context(), env.tenantID, codes.EntityLead)
	if err != nil {
		t.Fatalf("generate lead code: %v", err)
	}
	l, err := env.q.CreateLead(t.Context(), db.CreateLeadParams{
		TenantID:      env.tenantID,
		EntityCode:    &code,
		LeadName:      "Lead Mentah",
		LeadOwner:     &uid,
		LeadStatus:    "New",
		ContactPerson: ptr("Kontak Lead"),
		MobilePhone:   ptr("08123"),
		Whatsapp:      ptr("08124"),
		Email:         ptr("lead@example.com"),
		CreatedBy:     &uid,
	})
	if err != nil {
		t.Fatalf("seed lead: %v", err)
	}

	contacts, preselect, leadInfo, err := env.resolveActivityContacts(t, uid, "member", "sales", "lead", l.ID)
	if err != nil {
		t.Fatalf("activityContactsForTarget: %v", err)
	}
	if contacts != nil {
		t.Errorf("lead target tak boleh memuat kontak, got %+v", contacts)
	}
	if preselect != "" {
		t.Errorf("lead target tak boleh preselect, got %q", preselect)
	}
	if leadInfo == nil {
		t.Fatalf("lead target harus mengisi leadInfo")
	}
	if leadInfo.Name != "Kontak Lead" || leadInfo.Phone != "08123" ||
		leadInfo.WhatsApp != "08124" || leadInfo.Email != "lead@example.com" {
		t.Errorf("leadInfo tak sesuai field lead, got %+v", leadInfo)
	}
}

// TestActivityContactsForTarget_OutOfScope_ReturnsEmptyNotError: target di luar
// cakupan F3 aktor → kosong (nil,"",nil,nil), BUKAN error — form tetap tampil,
// field Kontak cuma kosong (pola sama targetInScope).
func TestActivityContactsForTarget_OutOfScope_ReturnsEmptyNotError(t *testing.T) {
	env, actor := setupAccounts(t)
	ownerB := env.seedMember(t, "ownerb-bl164@local", "member", 0).ID
	accB := env.seedAccount(t, "Desa Orang Lain", &ownerB, nil, nil)
	env.seedContact(t, accB.ID, "Kontak Orang Lain", &ownerB, false)

	contacts, preselect, leadInfo, err := env.resolveActivityContacts(t, actor, "member", "sales", "account", accB.ID)
	if err != nil {
		t.Fatalf("target di luar cakupan harus kosong, bukan error: %v", err)
	}
	if contacts != nil || preselect != "" || leadInfo != nil {
		t.Errorf("target di luar cakupan harus kembalikan kosong, got contacts=%+v preselect=%q leadInfo=%+v",
			contacts, preselect, leadInfo)
	}
}

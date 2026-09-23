package handler

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"go_starter/internal/codes"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_deals_sort_test.go — BL-157e: daftar Deals terurut ke-7 kolomnya via
// ?sort=col&dir=asc|desc, klon arsitektur BL-157a..d (subscriptions/leads/
// accounts/contacts _sort_test.go). Tak menguji ulang SEMUA kolom simetris —
// cukup representatif per pola cursor (NOT NULL teks: name/stage; nullable
// teks: code; nullable numeric: amount; nullable smallint: probability;
// nullable date: close; nullable via JOIN: owner) + fallback + F3 + mine-only
// + pagination, karena mekanisme cursor generik (sortcursor.go) sudah teruji
// tuntas di 157a.

// seedDealFull = varian seed deal dgn kontrol penuh atas kolom sortable
// nullable (amount/probability/closeDate/owner) — seedDeal (sales_quotes_test.go)
// tak cukup fleksibel utk uji posisi NULL. amount/probability/closeDate ""
// ≡ NULL.
func (e *testEnv) seedDealFull(
	t *testing.T, accountID int64, name string, owner *int64, stage, amount, probability, closeDate string,
) db.Deal {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityDeal)
	if err != nil {
		t.Fatalf("generate deal code: %v", err)
	}
	var amtNum pgtype.Numeric
	if amount != "" {
		amtNum = numFrom(t, amount)
	}
	var probPtr *int16
	if probability != "" {
		n, perr := strconv.ParseInt(probability, 10, 16)
		if perr != nil {
			t.Fatalf("parse probability %s: %v", probability, perr)
		}
		p := int16(n)
		probPtr = &p
	}
	var closeDt pgtype.Date
	if closeDate != "" {
		d, derr := time.Parse(dateLayout, closeDate)
		if derr != nil {
			t.Fatalf("parse close date %s: %v", closeDate, derr)
		}
		closeDt = pgtype.Date{Time: d, Valid: true}
	}
	d, err := e.q.CreateDeal(t.Context(), db.CreateDealParams{
		TenantID:          e.tenantID,
		EntityCode:        &code,
		DealName:          name,
		AccountID:         accountID,
		DealOwner:         owner,
		Stage:             stage,
		Amount:            amtNum,
		Probability:       probPtr,
		ExpectedCloseDate: closeDt,
		CreatedBy:         owner,
	})
	if err != nil {
		t.Fatalf("seed deal %s: %v", name, err)
	}
	return d
}

// seedDealNoCode = deal dgn entity_code NULL (GenerateEntityCode dilewati) —
// utk uji posisi NULL kolom "Kode".
func (e *testEnv) seedDealNoCode(t *testing.T, accountID int64, name string, owner *int64) db.Deal {
	t.Helper()
	d, err := e.q.CreateDeal(t.Context(), db.CreateDealParams{
		TenantID:  e.tenantID,
		DealName:  name,
		AccountID: accountID,
		DealOwner: owner,
		Stage:     dealInitialStage,
		CreatedBy: owner,
	})
	if err != nil {
		t.Fatalf("seed deal no-code %s: %v", name, err)
	}
	return d
}

func TestDealsList_SortNameAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedDealFull(t, acc.ID, "Deal Zebra", &uid, dealInitialStage, "", "", "")
	env.seedDealFull(t, acc.ID, "Deal Awal", &uid, dealInitialStage, "", "", "")
	env.seedDealFull(t, acc.ID, "Deal Mekar", &uid, dealInitialStage, "", "", "")

	req := accountsReq(http.MethodGet, "/w/test/deals?view=table&sort=name&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.DealsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "Deal Awal")
	iMekar := strings.Index(body, "Deal Mekar")
	iZebra := strings.Index(body, "Deal Zebra")
	if iAwal < 0 || iMekar < 0 || iZebra < 0 {
		t.Fatalf("ketiga deal harus tampil:\n%s", body)
	}
	if !(iAwal < iMekar && iMekar < iZebra) {
		t.Errorf("dir=asc harus urut Awal < Mekar < Zebra, dapat posisi %d/%d/%d", iAwal, iMekar, iZebra)
	}
}

func TestDealsList_SortNameDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedDealFull(t, acc.ID, "Deal Zebra", &uid, dealInitialStage, "", "", "")
	env.seedDealFull(t, acc.ID, "Deal Awal", &uid, dealInitialStage, "", "", "")

	req := accountsReq(http.MethodGet, "/w/test/deals?view=table&sort=name&dir=desc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.DealsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "Deal Awal")
	iZebra := strings.Index(body, "Deal Zebra")
	if iAwal < 0 || iZebra < 0 {
		t.Fatalf("kedua deal harus tampil:\n%s", body)
	}
	if !(iZebra < iAwal) {
		t.Errorf("dir=desc harus urut Zebra < Awal, dapat posisi %d/%d", iZebra, iAwal)
	}
}

// ?sort= tak dikenal → TAK error, jatuh ke jalur default (created_at DESC).
func TestDealsList_SortUnknownFallsBackToDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedDealFull(t, acc.ID, "Deal Fallback", &uid, dealInitialStage, "", "", "")

	req := accountsReq(http.MethodGet, "/w/test/deals?view=table&sort=bogus", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.DealsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Deal Fallback") {
		t.Errorf("?sort= tak dikenal harus tetap render daftar (jatuh ke default)")
	}
}

// Tahap (NOT NULL, raw enum alfabetis — bukan urutan tingkat pipeline).
func TestDealsList_SortStageAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedDealFull(t, acc.ID, "Deal StageNegotiation", &uid, "Negotiation", "", "", "")
	env.seedDealFull(t, acc.ID, "Deal StageProposal", &uid, "Proposal", "", "", "")
	env.seedDealFull(t, acc.ID, "Deal StageQualification", &uid, "Qualification", "", "", "")

	req := accountsReq(http.MethodGet, "/w/test/deals?view=table&sort=stage&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.DealsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iN := strings.Index(body, "Deal StageNegotiation")
	iP := strings.Index(body, "Deal StageProposal")
	iQ := strings.Index(body, "Deal StageQualification")
	if iN < 0 || iP < 0 || iQ < 0 {
		t.Fatalf("ketiga baris stage harus tampil:\n%s", body)
	}
	if !(iN < iP && iP < iQ) {
		t.Errorf("dir=asc harus urut Negotiation < Proposal < Qualification, dapat posisi %d/%d/%d", iN, iP, iQ)
	}
}

// Kode (entity_code nullable). asc→NULL akhir, desc→NULL awal.
func TestDealsList_SortCodeAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedDealFull(t, acc.ID, "Deal CodeAwal", &uid, dealInitialStage, "", "", "")
	env.seedDealFull(t, acc.ID, "Deal CodeZebra", &uid, dealInitialStage, "", "", "")
	env.seedDealNoCode(t, acc.ID, "Deal CodeNull", &uid)

	req := accountsReq(http.MethodGet, "/w/test/deals?view=table&sort=code&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.DealsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Deal CodeAwal")
	iZ := strings.Index(body, "Deal CodeZebra")
	iN := strings.Index(body, "Deal CodeNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	// entity_code dialokasikan berurutan (DEAL-001, DEAL-002, ...) → Awal < Zebra.
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut kode-Awal < kode-Zebra < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}

func TestDealsList_SortCodeDescNullsFirst(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sort", &uid, nil, nil)
	env.seedDealFull(t, acc.ID, "Deal CodeAwal", &uid, dealInitialStage, "", "", "")
	env.seedDealFull(t, acc.ID, "Deal CodeZebra", &uid, dealInitialStage, "", "", "")
	env.seedDealNoCode(t, acc.ID, "Deal CodeNull", &uid)

	req := accountsReq(http.MethodGet, "/w/test/deals?view=table&sort=code&dir=desc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.DealsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Deal CodeAwal")
	iZ := strings.Index(body, "Deal CodeZebra")
	iN := strings.Index(body, "Deal CodeNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iN < iZ && iZ < iA) {
		t.Errorf("dir=desc harus urut NULL(awal) < kode-Zebra < kode-Awal, dapat posisi %d/%d/%d", iN, iZ, iA)
	}
}

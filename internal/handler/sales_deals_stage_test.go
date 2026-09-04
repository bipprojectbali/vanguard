package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// sales_deals_stage_test.go — BL-21: deal Closed Won → auto-create Subscription.
// Sebelumnya langganan HANYA lahir dari renewal + seed (deals.created_subscription_id
// tak pernah terisi). Di sini dijaga: (1) Won melahirkan TEPAT satu langganan tertaut
// dua-arah dgn status pilihan user, (2) re-Won tak menggandakan (idempoten), (3) konflik
// one-active ditolak (?err), (4) ATOMIK — gagal/validasi tak lolos → deal TETAP stage lama
// (tak ada langganan setengah jadi). Koneksi test = superuser (bypass RLS) → uji LOGIKA
// handler; peran admin (crm:*, ScopeAll) memisahkan alur BL-21 dari gate F2/F3.

// seedWonReadyDeal menaruh satu deal SIAP-menang: punya plan + amount + termin, di stage
// Negotiation (dalam pipeline, belum terminal). Field inilah yang diturunkan langganan
// saat Closed Won; melewatkan salah satunya = jalur validasi (diuji terpisah).
func (e *testEnv) seedWonReadyDeal(
	t *testing.T, accountID int64, owner *int64, planID int64, amount, term string,
) db.Deal {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityDeal)
	if err != nil {
		t.Fatalf("generate deal code: %v", err)
	}
	d, err := e.q.CreateDeal(t.Context(), db.CreateDealParams{
		TenantID:         e.tenantID,
		EntityCode:       &code,
		DealName:         "Deal Menang",
		AccountID:        accountID,
		DealOwner:        owner,
		PlanRequestedID:  &planID,
		Stage:            "Negotiation",
		Amount:           numFrom(t, amount),
		SubscriptionTerm: &term,
		CreatedBy:        owner,
	})
	if err != nil {
		t.Fatalf("seed won-ready deal: %v", err)
	}
	return d
}

// postDealStage menjalankan DealStage (POST .../stage) sbg admin (write, ScopeAll)
// dgn form yang diberikan; mengembalikan recorder utuh (Location + status).
func (e *testEnv) postDealStage(uid int64, dealID int64, form url.Values) *httptest.ResponseRecorder {
	req := accountsReq(http.MethodPost, "/w/test/deals/"+itoa(dealID)+"/stage", form, itoa(dealID))
	return e.runAccount(uid, "owner", "admin", req, e.h.DealStage)
}

// wonForm = form Closed Won lengkap (alasan menang + status langganan awal).
func wonForm(status string) url.Values {
	return url.Values{
		"stage":               {"Closed Won"},
		"win_loss_reason":     {"Menang tender"},
		"subscription_status": {status},
	}
}

// freshDeal memuat ulang deal dari DB (buktikan stage & tautan langganan PASCA-aksi).
func (e *testEnv) freshDeal(t *testing.T, id int64) db.Deal {
	t.Helper()
	d, err := e.q.GetDeal(t.Context(), id)
	if err != nil {
		t.Fatalf("get deal %d: %v", id, err)
	}
	return d
}

// countSubsForAccount menghitung langganan hidup satu desa (buktikan tak ada dobel).
func (e *testEnv) countSubsForAccount(t *testing.T, accountID int64) int {
	t.Helper()
	var n int
	if err := e.h.Pool.QueryRow(t.Context(),
		`SELECT count(*) FROM subscriptions WHERE account_id=$1 AND deleted_at IS NULL`,
		accountID).Scan(&n); err != nil {
		t.Fatalf("count subs: %v", err)
	}
	return n
}

// TestDealWon_CreatesActiveSubscription: happy-path. Closed Won (status Active) →
// TEPAT satu langganan lahir, tertaut dua-arah (source_deal_id ↔ created_subscription_id),
// field diturunkan benar (owner, plan, billing, termin, MRR=amount/bulan, ARR=MRR×12).
func TestDealWon_CreatesActiveSubscription(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Menang", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Inti", "PLN-WON", "100000.00")
	// Annual = 12 bulan; amount 12jt/termin → MRR 1jt, ARR 12jt.
	deal := env.seedWonReadyDeal(t, acc.ID, &uid, plan, "12000000", "Annual")

	rec := env.postDealStage(uid, deal.ID, wonForm("Active"))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=staged") {
		t.Fatalf("Won harus sukses (?ok=staged), got %q (status %d)\n%s",
			loc, rec.Code, rec.Body.String())
	}

	got := env.freshDeal(t, deal.ID)
	if got.Stage != "Closed Won" {
		t.Errorf("stage deal = %q, want Closed Won", got.Stage)
	}
	if got.CreatedSubscriptionID == nil {
		t.Fatalf("deal.created_subscription_id harus terisi (tautan balik)")
	}
	if n := env.countSubsForAccount(t, acc.ID); n != 1 {
		t.Fatalf("langganan untuk desa = %d, want 1", n)
	}

	sub, err := env.q.GetSubscription(t.Context(), *got.CreatedSubscriptionID)
	if err != nil {
		t.Fatalf("get subscription: %v", err)
	}
	if sub.SourceDealID == nil || *sub.SourceDealID != deal.ID {
		t.Errorf("sub.source_deal_id = %v, want %d", sub.SourceDealID, deal.ID)
	}
	if sub.Status != "Active" {
		t.Errorf("sub.status = %q, want Active", sub.Status)
	}
	if sub.PlanID != plan {
		t.Errorf("sub.plan_id = %d, want %d", sub.PlanID, plan)
	}
	if sub.SubscriptionOwner == nil || *sub.SubscriptionOwner != uid {
		t.Errorf("sub.owner = %v, want %d (deal owner)", sub.SubscriptionOwner, uid)
	}
	if sub.BillingCycle == nil || *sub.BillingCycle != "Annual" {
		t.Errorf("sub.billing_cycle = %v, want Annual", sub.BillingCycle)
	}
	if sub.ContractTermMonths == nil || *sub.ContractTermMonths != 12 {
		t.Errorf("sub.contract_term_months = %v, want 12", sub.ContractTermMonths)
	}
	if sub.AutoRenew {
		t.Errorf("sub.auto_renew = true, want false (perpanjangan lewat renewal)")
	}
	if ratFromNumeric(sub.Mrr).Cmp(ratFromNumeric(numFrom(t, "1000000"))) != 0 {
		t.Errorf("sub.mrr = %s, want 1000000 (amount/12)", formatRupiah(sub.Mrr))
	}
	if ratFromNumeric(sub.Arr).Cmp(ratFromNumeric(numFrom(t, "12000000"))) != 0 {
		t.Errorf("sub.arr = %s, want 12000000 (mrr×12)", formatRupiah(sub.Arr))
	}
	if !sub.StartDate.Valid || !sub.EndDate.Valid || !sub.EndDate.Time.After(sub.StartDate.Time) {
		t.Errorf("start/end date invalid: start=%v end=%v", sub.StartDate, sub.EndDate)
	}
}

// TestDealWon_TrialCoexistsWithActive: status awal Trial (keputusan a) TAK terganjal
// idx_subs_one_active (indeks hanya menggigit status Active) — Trial boleh koeksis dgn
// langganan Active desa yang sama. Buktikan langganan Trial lahir walau sudah ada Active.
func TestDealWon_TrialCoexistsWithActive(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Trial", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Inti", "PLN-TRIAL", "100000.00")
	env.seedDashboardSub(t, acc.ID, plan, &uid, "Active", "6000000", nil) // Active eksisting
	deal := env.seedWonReadyDeal(t, acc.ID, &uid, plan, "6000000", "Monthly")

	rec := env.postDealStage(uid, deal.ID, wonForm("Trial"))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=staged") {
		t.Fatalf("Won(Trial) harus sukses, got %q (status %d)", loc, rec.Code)
	}
	got := env.freshDeal(t, deal.ID)
	if got.CreatedSubscriptionID == nil {
		t.Fatalf("langganan Trial harus lahir & tertaut")
	}
	sub, err := env.q.GetSubscription(t.Context(), *got.CreatedSubscriptionID)
	if err != nil {
		t.Fatalf("get subscription: %v", err)
	}
	if sub.Status != "Trial" {
		t.Errorf("sub.status = %q, want Trial", sub.Status)
	}
	if n := env.countSubsForAccount(t, acc.ID); n != 2 {
		t.Errorf("langganan desa = %d, want 2 (Active eksisting + Trial baru)", n)
	}
}

// TestDealWon_Idempotent: deal yang SUDAH menautkan langganan, di-Won lagi (Won→…→Won)
// TAK menggandakan — subscriptionFromWonDeal skip idempoten, tautan lama bertahan.
func TestDealWon_Idempotent(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Ulang", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Inti", "PLN-IDEM", "100000.00")
	deal := env.seedWonReadyDeal(t, acc.ID, &uid, plan, "12000000", "Annual")

	if rec := env.postDealStage(uid, deal.ID, wonForm("Active")); !strings.Contains(
		rec.Header().Get("Location"), "ok=staged") {
		t.Fatalf("Won pertama harus sukses, got %q", rec.Header().Get("Location"))
	}
	firstSubID := *env.freshDeal(t, deal.ID).CreatedSubscriptionID

	// Won kedua (mis. dipindah ke Negotiation lalu Won lagi lewat UI) → tetap 1 langganan.
	if rec := env.postDealStage(uid, deal.ID, wonForm("Active")); !strings.Contains(
		rec.Header().Get("Location"), "ok=staged") {
		t.Fatalf("Won kedua harus tetap 200/redirect sukses, got %q", rec.Header().Get("Location"))
	}
	if got := env.freshDeal(t, deal.ID); *got.CreatedSubscriptionID != firstSubID {
		t.Errorf("created_subscription_id berubah (%d → %d) — tak boleh dobel",
			firstSubID, *got.CreatedSubscriptionID)
	}
	if n := env.countSubsForAccount(t, acc.ID); n != 1 {
		t.Errorf("langganan desa = %d, want 1 (idempoten)", n)
	}
}

// TestDealWon_ActiveConflictRejected: sudah ada langganan Active utk desa+paket ini →
// Won(Active) DITOLAK (?err=sub_active_exists, keputusan c) dan ATOMIK — deal TETAP
// stage lama, tak ada langganan baru (perpanjangan lewat renewal, bukan deal).
func TestDealWon_ActiveConflictRejected(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Bentrok", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Inti", "PLN-CONF", "100000.00")
	env.seedDashboardSub(t, acc.ID, plan, &uid, "Active", "6000000", nil)
	deal := env.seedWonReadyDeal(t, acc.ID, &uid, plan, "12000000", "Annual")

	rec := env.postDealStage(uid, deal.ID, wonForm("Active"))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=sub_active_exists") {
		t.Fatalf("konflik one-active harus ?err=sub_active_exists, got %q", loc)
	}
	if got := env.freshDeal(t, deal.ID); got.Stage != "Negotiation" {
		t.Errorf("ATOMIK: deal tak boleh jadi Won saat create gagal, stage = %q", got.Stage)
	}
	if got := env.freshDeal(t, deal.ID); got.CreatedSubscriptionID != nil {
		t.Errorf("tak boleh ada tautan langganan saat ditolak")
	}
	if n := env.countSubsForAccount(t, acc.ID); n != 1 {
		t.Errorf("langganan desa = %d, want 1 (hanya yang eksisting)", n)
	}
}

// TestDealWon_ValidationAtomic: tiap prasyarat yang tak terpenuhi menolak Won lewat PRG
// (?err=CODE) dan ATOMIK — deal TETAP Negotiation, NOL langganan lahir. Mencakup
// plan_required (paket wajib), term_required (termin menurunkan nilai), sub_status
// (status awal wajib Trial/Active), win_loss (alasan wajib saat terminal — perilaku lama).
func TestDealWon_ValidationAtomic(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Validasi", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Inti", "PLN-VAL", "100000.00")

	// seedDeal builder per-kasus: sebagian sengaja tak lengkap.
	fullForm := wonForm("Active")
	badStatusForm := wonForm("Suspended") // di luar Trial/Active
	noWinLoss := url.Values{"stage": {"Closed Won"}, "subscription_status": {"Active"}}

	cases := []struct {
		name    string
		deal    db.Deal
		form    url.Values
		wantErr string
	}{
		{
			name:    "plan_required",
			deal:    env.seedReportDeal(t, acc.ID, &uid, "Negotiation", "12000000"), // tanpa plan/termin
			form:    fullForm,
			wantErr: "err=plan_required",
		},
		{
			name:    "term_required",
			deal:    env.seedDealWithPlan(t, acc.ID, &uid, plan), // plan ada, termin kosong
			form:    fullForm,
			wantErr: "err=term_required",
		},
		{
			name:    "sub_status",
			deal:    env.seedWonReadyDeal(t, acc.ID, &uid, plan, "12000000", "Annual"),
			form:    badStatusForm,
			wantErr: "err=sub_status",
		},
		{
			name:    "win_loss",
			deal:    env.seedWonReadyDeal(t, acc.ID, &uid, plan, "12000000", "Annual"),
			form:    noWinLoss,
			wantErr: "err=win_loss",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := env.postDealStage(uid, tc.deal.ID, tc.form)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, tc.wantErr) {
				t.Fatalf("want ?%s, got %q (status %d)", tc.wantErr, loc, rec.Code)
			}
			got := env.freshDeal(t, tc.deal.ID)
			if got.Stage == "Closed Won" {
				t.Errorf("ATOMIK: deal tak boleh Won saat %s", tc.name)
			}
			if got.CreatedSubscriptionID != nil {
				t.Errorf("tak boleh ada langganan saat %s", tc.name)
			}
		})
	}
}

// seedDealWithPlan = deal dgn plan tapi TANPA termin (jalur term_required).
func (e *testEnv) seedDealWithPlan(t *testing.T, accountID int64, owner *int64, planID int64) db.Deal {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityDeal)
	if err != nil {
		t.Fatalf("generate deal code: %v", err)
	}
	d, err := e.q.CreateDeal(t.Context(), db.CreateDealParams{
		TenantID:        e.tenantID,
		EntityCode:      &code,
		DealName:        "Deal Tanpa Termin",
		AccountID:       accountID,
		DealOwner:       owner,
		PlanRequestedID: &planID,
		Stage:           "Negotiation",
		Amount:          numFrom(t, "12000000"),
		CreatedBy:       owner,
	})
	if err != nil {
		t.Fatalf("seed deal with plan: %v", err)
	}
	return d
}

// TestWonSubStatusOptions_MirrorValid: dropdown status (wonSubStatusOptions) WAJIB cermin
// himpunan status yang divalidasi backend (validInitialSubStatuses) — kalau tidak, UI
// menawarkan pilihan yang ditolak, atau menyembunyikan pilihan yang sah. Guard murni-data.
func TestWonSubStatusOptions_MirrorValid(t *testing.T) {
	if len(wonSubStatusOptions) != len(validInitialSubStatuses) {
		t.Fatalf("opsi dropdown (%d) ≠ status valid (%d)",
			len(wonSubStatusOptions), len(validInitialSubStatuses))
	}
	for _, s := range wonSubStatusOptions {
		if _, ok := validInitialSubStatuses[s]; !ok {
			t.Errorf("opsi %q tak ada di validInitialSubStatuses (akan ditolak backend)", s)
		}
	}
}

// ── BL-44 (3a): picklist loss_reason_code saat Closed Lost ───────────────────

// lostForm = form Closed Lost. win_loss_reason tetap wajib (terminal, perilaku
// lama); loss_reason_code opsional di param agar jalur "kosong" bisa diuji.
func lostForm(code string) url.Values {
	f := url.Values{
		"stage":           {"Closed Lost"},
		"win_loss_reason": {"Kalah tender"},
	}
	if code != "" {
		f.Set("loss_reason_code", code)
	}
	return f
}

// TestDealLost_StoresLossReasonCode: Closed Lost dengan kode valid → stage
// tersimpan Closed Lost + loss_reason_code terekam (dipakai grouping laporan).
func TestDealLost_StoresLossReasonCode(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Kalah", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Negotiation", "5000000")

	rec := env.postDealStage(uid, deal.ID, lostForm("Kompetitor"))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=staged") {
		t.Fatalf("Closed Lost dgn kode valid harus sukses, got %q (status %d)", loc, rec.Code)
	}
	got := env.freshDeal(t, deal.ID)
	if got.Stage != "Closed Lost" {
		t.Errorf("stage = %q, want Closed Lost", got.Stage)
	}
	if got.LossReasonCode == nil || *got.LossReasonCode != "Kompetitor" {
		t.Errorf("loss_reason_code = %v, want Kompetitor", got.LossReasonCode)
	}
}

// TestDealLost_RequiresLossReasonCode: Closed Lost tanpa kode → ?err=loss_reason
// dan deal TETAP stage lama (validasi handler, bukan CHECK DB).
func TestDealLost_RequiresLossReasonCode(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Tanpa Kode", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Negotiation", "5000000")

	rec := env.postDealStage(uid, deal.ID, lostForm(""))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=loss_reason") {
		t.Fatalf("kode kosong harus ?err=loss_reason, got %q", loc)
	}
	if got := env.freshDeal(t, deal.ID); got.Stage != "Negotiation" {
		t.Errorf("deal tak boleh pindah stage saat kode kosong, stage = %q", got.Stage)
	}
}

// TestDealLost_RejectsInvalidLossReasonCode: kode di luar picklist ditolak
// (?err=loss_reason) — backend penjaga, bukan sekadar dropdown UI.
func TestDealLost_RejectsInvalidLossReasonCode(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Kode Ngawur", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Negotiation", "5000000")

	rec := env.postDealStage(uid, deal.ID, lostForm("Diskon"))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=loss_reason") {
		t.Fatalf("kode invalid harus ?err=loss_reason, got %q", loc)
	}
	if got := env.freshDeal(t, deal.ID); got.LossReasonCode != nil {
		t.Errorf("kode invalid tak boleh tersimpan, got %v", got.LossReasonCode)
	}
}

// TestDealReopen_ClearsLossReasonCode: deal Closed Lost (berkode) dibuka kembali
// ke stage aktif → loss_reason_code dibersihkan (kode hanya relevan saat kalah).
func TestDealReopen_ClearsLossReasonCode(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Reopen", &uid, nil, nil)
	deal := env.seedReportDeal(t, acc.ID, &uid, "Negotiation", "5000000")

	if rec := env.postDealStage(uid, deal.ID, lostForm("Anggaran")); !strings.Contains(
		rec.Header().Get("Location"), "ok=staged") {
		t.Fatalf("Closed Lost awal harus sukses, got %q", rec.Header().Get("Location"))
	}
	// Buka kembali ke stage aktif — tak butuh alasan.
	if rec := env.postDealStage(uid, deal.ID, url.Values{"stage": {"Proposal"}}); !strings.Contains(
		rec.Header().Get("Location"), "ok=staged") {
		t.Fatalf("reopen ke Proposal harus sukses, got %q", rec.Header().Get("Location"))
	}
	got := env.freshDeal(t, deal.ID)
	if got.Stage != "Proposal" {
		t.Errorf("stage = %q, want Proposal", got.Stage)
	}
	if got.LossReasonCode != nil {
		t.Errorf("loss_reason_code harus null pasca-reopen, got %v", got.LossReasonCode)
	}
}

// TestLossReasonCodeOptions_MirrorValid: dropdown (lossReasonCodeOptions) WAJIB
// cermin himpunan yang divalidasi backend (validLossReasonCodes) — kalau tidak,
// UI menawarkan pilihan yang ditolak, atau menyembunyikan yang sah.
func TestLossReasonCodeOptions_MirrorValid(t *testing.T) {
	if len(lossReasonCodeOptions) != len(validLossReasonCodes) {
		t.Fatalf("opsi (%d) ≠ kode valid (%d)", len(lossReasonCodeOptions), len(validLossReasonCodes))
	}
	for _, s := range lossReasonCodeOptions {
		if _, ok := validLossReasonCodes[s]; !ok {
			t.Errorf("opsi %q tak ada di validLossReasonCodes (akan ditolak backend)", s)
		}
	}
}

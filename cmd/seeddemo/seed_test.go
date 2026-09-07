package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"go_starter/internal/db"
	"go_starter/internal/testdb"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seed_test.go — beda dari precedent cmd/seedsubs (tanpa test): tugas ini
// punya bar korektnes eksplisit ("data dummy harus terlihat real") yang
// assertable, jadi test memverifikasi bukan cuma "tidak error", tapi bahwa
// distribusi yang disengaja (renewals due, SLA breach, health status
// campuran, seluruh stage deal) benar-benar mendarat di DB.

var pkgPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()
	pool, err := testdb.Pool(ctx, "seeddemo")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testdb:", err)
		os.Exit(1)
	}
	pkgPool = pool

	code := m.Run()

	if pool != nil {
		pool.Close()
		testdb.Drop(ctx, "seeddemo")
	}
	os.Exit(code)
}

func TestSeedInto(t *testing.T) {
	if pkgPool == nil {
		t.Skip("TEST_DATABASE_URL tidak di-set; lewati test seeddemo")
	}
	ctx := context.Background()

	// Tenant test langsung via CreateTenant (bukan WithSuper/GetPrimaryTenant)
	// karena ini bukan workspace primer — pola sama internal/db/entity_codes_test.go.
	ten, err := db.New(pkgPool).CreateTenant(ctx, db.CreateTenantParams{
		Name: "Seed Demo Test", Slug: "seed-demo-test",
	})
	if err != nil {
		t.Fatalf("buat tenant test: %v", err)
	}
	tenantID := ten.ID

	var stats seedStats
	if err := db.WithTenant(ctx, pkgPool, tenantID, func(q *db.Queries) error {
		s, err := seedInto(ctx, q, tenantID, "0101-000000")
		stats = s
		return err
	}); err != nil {
		t.Fatalf("seedInto: %v", err)
	}

	// 1. Jumlah baris tiap tabel kunci > 0.
	tableCounts := map[string]int{
		"accounts": stats.Accounts, "contacts": stats.Contacts, "leads": stats.Leads,
		"deals": stats.Deals, "quotes": stats.Quotes, "quote_items": stats.QuoteItems,
		"subscriptions": stats.Subscriptions, "customer_success": stats.CustomerSuccess,
		"activities": stats.Activities, "tickets": stats.Tickets,
		"engagements": stats.Engagements, "success_plans": stats.SuccessPlans,
		"cs_impl_tasks": stats.ImplTasks, "cs_trainings": stats.Trainings,
		"plans": stats.Plans, "sla_policies": stats.SLAPolicies,
		"playbooks": stats.Playbooks, "kb_articles": stats.KBArticles,
	}
	for table, got := range tableCounts {
		if got <= 0 {
			t.Errorf("stats.%s = %d, ingin > 0", table, got)
		}
	}

	// Silang-cek ke DB langsung (bukan cuma percaya nilai stats yang dikembalikan
	// fungsi seed) — pola sama internal/handler/rls_test.go (raw COUNT(*)).
	assertCount := func(table string) {
		t.Helper()
		var n int
		q := fmt.Sprintf("SELECT count(*) FROM %s WHERE tenant_id = $1", table)
		if err := pkgPool.QueryRow(ctx, q, tenantID).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n <= 0 {
			t.Errorf("SELECT count(*) FROM %s = %d, ingin > 0", table, n)
		}
	}
	for _, table := range []string{
		"accounts", "contacts", "leads", "deals", "quotes", "quote_items",
		"subscriptions", "customer_success", "activities", "tickets",
		"engagements", "success_plans", "cs_impl_tasks", "cs_trainings",
		"plans", "sla_policies", "playbooks", "kb_articles",
	} {
		assertCount(table)
	}

	// 2. ≥1 subscription Active dengan end_date di jendela 30 hari (Renewals Due).
	var renewalsDue int
	if err := pkgPool.QueryRow(ctx,
		`SELECT count(*) FROM subscriptions
		 WHERE tenant_id = $1 AND status = 'Active'
		   AND end_date BETWEEN CURRENT_DATE AND CURRENT_DATE + 30`,
		tenantID).Scan(&renewalsDue); err != nil {
		t.Fatalf("count renewals due: %v", err)
	}
	if renewalsDue == 0 {
		t.Error("tidak ada subscription Active dgn end_date di jendela 30 hari (Renewals Due)")
	}

	// 3. ≥1 ticket TERBUKA dengan sla_deadline_at < sekarang (breach).
	var breached int
	if err := pkgPool.QueryRow(ctx,
		`SELECT count(*) FROM tickets
		 WHERE tenant_id = $1 AND status != 'selesai' AND sla_deadline_at < now()`,
		tenantID).Scan(&breached); err != nil {
		t.Fatalf("count tickets breach: %v", err)
	}
	if breached == 0 {
		t.Error("tidak ada ticket terbuka dgn sla_deadline_at di masa lalu (SLA breach)")
	}

	// 4a. customer_success.health_status mencakup Healthy/At-Risk/Critical.
	rows, err := pkgPool.Query(ctx,
		`SELECT DISTINCT health_status FROM customer_success
		 WHERE tenant_id = $1 AND health_status IS NOT NULL`, tenantID)
	if err != nil {
		t.Fatalf("query health_status distinct: %v", err)
	}
	seen := map[string]bool{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan health_status: %v", err)
		}
		seen[s] = true
	}
	rows.Close()
	for _, want := range []string{"Healthy", "At-Risk", "Critical"} {
		if !seen[want] {
			t.Errorf("health_status %q tidak ditemukan di customer_success", want)
		}
	}

	// 4b. Ada desa TANPA baris customer_success sama sekali (bucket "Belum Dinilai").
	var uncovered int
	if err := pkgPool.QueryRow(ctx,
		`SELECT count(*) FROM accounts a
		 LEFT JOIN customer_success cs ON cs.account_id = a.id
		 WHERE a.tenant_id = $1 AND cs.id IS NULL`, tenantID).Scan(&uncovered); err != nil {
		t.Fatalf("count accounts tanpa customer_success: %v", err)
	}
	if uncovered == 0 {
		t.Error("semua desa punya baris customer_success — seharusnya ada yg tanpa baris sama sekali")
	}

	// 5. deals.stage mencakup ketujuh nilai (5 terbuka + 2 closed).
	stageRows, err := pkgPool.Query(ctx,
		`SELECT DISTINCT stage FROM deals WHERE tenant_id = $1`, tenantID)
	if err != nil {
		t.Fatalf("query stage distinct: %v", err)
	}
	stages := map[string]bool{}
	for stageRows.Next() {
		var s string
		if err := stageRows.Scan(&s); err != nil {
			t.Fatalf("scan stage: %v", err)
		}
		stages[s] = true
	}
	stageRows.Close()
	for _, want := range []string{
		"Prospecting", "Qualification", "Demo", "Proposal", "Negotiation",
		"Closed Won", "Closed Lost",
	} {
		if !stages[want] {
			t.Errorf("deal stage %q tidak ditemukan", want)
		}
	}

	// 6. village_code = kode Kemendagri ASLI 4-segmen dari master Desa (regions
	// level 4, BL-66), format NN.NN.NN.NNNN — BUKAN prefix "VC-"/"VC-LEAD-" lama
	// maupun segmen ke-4 fiktif. Mengunci bahwa seed menarik dari villagePool
	// (regions.go), bukan merakit kode sendiri.
	villageCodeRe := regexp.MustCompile(`^[0-9]{2}\.[0-9]{2}\.[0-9]{2}\.[0-9]{4}$`)
	vcRows, err := pkgPool.Query(ctx,
		`SELECT village_code FROM accounts WHERE tenant_id = $1 AND village_code IS NOT NULL`, tenantID)
	if err != nil {
		t.Fatalf("query village_code: %v", err)
	}
	var villageCodeCount int
	for vcRows.Next() {
		var code string
		if err := vcRows.Scan(&code); err != nil {
			t.Fatalf("scan village_code: %v", err)
		}
		villageCodeCount++
		if strings.Contains(code, "VC-") {
			t.Errorf("village_code %q masih pakai prefix VC-/VC-LEAD- lama", code)
		}
		if !villageCodeRe.MatchString(code) {
			t.Errorf("village_code %q tidak berformat pseudo-Kemendagri NN.NN.NN.NNNN", code)
		}
	}
	vcRows.Close()
	if villageCodeCount == 0 {
		t.Error("tidak ada account dgn village_code terisi — seharusnya semua desa demo punya kode")
	}

	// 7. Status aktivitas mengikuti PARTISI per-kind seperti form UI (BL-72):
	// task ∈ {Not Started, In Progress, Completed, Deferred}; meeting ∈
	// {Planned, Held, Cancelled, No-Show}; call/chat/note TAK ber-field status →
	// NULL. Sebelum fix, seed memilih acak dari gabungan semua status sehingga
	// mis. Pertemuan bisa ber-"Deferred" (data tak realistis, mustahil via UI).
	allowedByKind := map[string]map[string]bool{
		"task":    {"Not Started": true, "In Progress": true, "Completed": true, "Deferred": true},
		"meeting": {"Planned": true, "Held": true, "Cancelled": true, "No-Show": true},
	}
	actRows, err := pkgPool.Query(ctx,
		`SELECT kind, status FROM activities WHERE tenant_id = $1`, tenantID)
	if err != nil {
		t.Fatalf("query activities kind/status: %v", err)
	}
	var seenTaskStatus, seenMeetingStatus bool
	for actRows.Next() {
		var kind string
		var status *string
		if err := actRows.Scan(&kind, &status); err != nil {
			t.Fatalf("scan activity kind/status: %v", err)
		}
		switch kind {
		case "task", "meeting":
			if status == nil {
				t.Errorf("aktivitas kind %q ber-status NULL — seharusnya terisi", kind)
				continue
			}
			if !allowedByKind[kind][*status] {
				t.Errorf("aktivitas kind %q ber-status %q di luar partisi UI", kind, *status)
			}
			if kind == "task" {
				seenTaskStatus = true
			} else {
				seenMeetingStatus = true
			}
		case "call", "chat", "note":
			if status != nil {
				t.Errorf("aktivitas kind %q ber-status %q — form tak punya field status, seharusnya NULL", kind, *status)
			}
		}
	}
	actRows.Close()
	if !seenTaskStatus {
		t.Error("tidak ada aktivitas task ber-status — partisi status task tak terverifikasi")
	}
	if !seenMeetingStatus {
		t.Error("tidak ada aktivitas meeting ber-status — partisi status meeting tak terverifikasi")
	}
}

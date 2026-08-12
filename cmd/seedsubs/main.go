// Command seedsubs mengisi DB dev dengan data contoh Modul 5 (Subscriptions) agar
// dasbor Renewals & Churn langsung terlihat berisi. BUKAN untuk produksi.
//
// Jalankan dari worktree yang punya .env (mis. folder utama):
//
//	go run ./cmd/seedsubs                 # ke workspace primer
//	go run ./cmd/seedsubs -slug my-space  # ke workspace tertentu
//
// Idempotent-friendly: tiap run memakai tag waktu unik pada plan_code & nama desa
// sehingga rerun tak bentrok idx_plans_code / entity_code. Semua langganan Active
// dibuat di (desa, paket) berbeda agar lolos idx_subs_one_active (1 Active per
// tenant+account+plan). Data ini bertanda seed — aman dihapus manual kapan saja.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"go_starter/internal/codes"
	"go_starter/internal/config"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	slug := flag.String("slug", "", "slug workspace tujuan (kosong = workspace primer)")
	flag.Parse()

	if err := run(*slug); err != nil {
		log.Fatalf("seedsubs: %v", err)
	}
}

func run(slug string) error {
	ctx := context.Background()
	if err := config.LoadDotEnv(".env"); err != nil {
		return fmt.Errorf("load .env: %w", err)
	}
	cfg, err := config.LoadMigrateConfig()
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open pool: %w", err)
	}
	defer pool.Close()

	tenantID, tenantName, err := resolveTenant(ctx, pool, slug)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "seedsubs: menargetkan workspace %q (tenant #%d)\n", tenantName, tenantID)

	tag := time.Now().Format("0102-150405") // pembeda run: MMDD-HHMMSS (test-only)
	return db.WithTenant(ctx, pool, tenantID, func(q *db.Queries) error {
		return seedInto(ctx, q, tenantID, tag)
	})
}

// resolveTenant memilih tenant tujuan: by-slug bila diberi, selain itu workspace
// primer (rumah aplikasi). Dibaca via WithSuper (tenants di luar RLS).
func resolveTenant(ctx context.Context, pool *pgxpool.Pool, slug string) (int64, string, error) {
	var id int64
	var name string
	err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
		var (
			t   db.Tenant
			err error
		)
		if slug == "" {
			t, err = q.GetPrimaryTenant(ctx)
		} else {
			t, err = q.GetTenantBySlug(ctx, slug)
		}
		if err != nil {
			return err
		}
		id, name = t.ID, t.Name
		return nil
	})
	if err != nil {
		return 0, "", fmt.Errorf("cari tenant: %w", err)
	}
	return id, name, nil
}

// seedInto membuat 3 plan katalog lalu langganan contoh di empat kondisi dasbor:
// jatuh tempo (due), masa tenggang (grace), churn sukarela, churn terpaksa.
func seedInto(ctx context.Context, q *db.Queries, tenantID int64, tag string) error {
	plans, err := seedPlans(ctx, q, tenantID, tag)
	if err != nil {
		return err
	}

	today := time.Now()
	// (label, offsetHariEndDate, mrr, plan) — Active untuk dasbor Renewals.
	active := []struct {
		label  string
		offset int
		mrr    string
		plan   int64
	}{
		{"Due-A", 15, "500000", plans[0]},   // jatuh tempo dalam 30 hari
		{"Due-B", 7, "750000", plans[1]},    // jatuh tempo dekat
		{"Due-C", 28, "1200000", plans[2]},  // jatuh tempo hampir batas
		{"Grace-A", -5, "600000", plans[0]}, // lewat tempo (masa tenggang)
		{"Grace-B", -20, "450000", plans[1]},
	}
	for _, a := range active {
		acc, err := seedAccount(ctx, q, tenantID, fmt.Sprintf("Desa %s %s", a.label, tag))
		if err != nil {
			return err
		}
		if _, err := createActive(ctx, q, tenantID, acc, a.plan, a.mrr, today.AddDate(0, 0, a.offset)); err != nil {
			return err
		}
	}

	// Churn untuk dasbor Churn: sukarela & terpaksa, dgn MRR hilang + alasan.
	churn := []struct {
		label, typ, reason, mrr string
		plan                    int64
	}{
		{"Churn-Vol-A", "Voluntary", "Budget", "500000", plans[0]},
		{"Churn-Vol-B", "Voluntary", "Competitor", "800000", plans[1]},
		{"Churn-Invol", "Involuntary", "No Adoption", "650000", plans[2]},
	}
	for _, c := range churn {
		acc, err := seedAccount(ctx, q, tenantID, fmt.Sprintf("Desa %s %s", c.label, tag))
		if err != nil {
			return err
		}
		if err := createChurned(ctx, q, tenantID, acc, c.plan, c.typ, c.reason, c.mrr); err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stderr, "seedsubs: selesai — %d plan, %d langganan aktif, %d churn.\n",
		len(plans), len(active), len(churn))
	return nil
}

// seedPlans membuat tiga plan katalog (satu per kategori) dengan kode unik-run.
func seedPlans(ctx context.Context, q *db.Queries, tenantID int64, tag string) ([]int64, error) {
	specs := []struct{ name, code, category, price string }{
		{"Paket Inti " + tag, "SEED-CORE-" + tag, "Core", "500000"},
		{"Paket Tambahan " + tag, "SEED-ADD-" + tag, "Add-on", "250000"},
		{"Paket Modul " + tag, "SEED-MOD-" + tag, "Module", "1000000"},
	}
	ids := make([]int64, 0, len(specs))
	for _, s := range specs {
		price, err := num(s.price)
		if err != nil {
			return nil, err
		}
		p, err := q.CreatePlan(ctx, db.CreatePlanParams{
			TenantID:     tenantID,
			PlanName:     s.name,
			PlanCode:     s.code,
			PlanCategory: s.category,
			IsActive:     true,
			BasePrice:    price,
			Currency:     "IDR",
		})
		if err != nil {
			return nil, fmt.Errorf("buat plan %s: %w", s.code, err)
		}
		ids = append(ids, p.ID)
	}
	return ids, nil
}

// seedAccount membuat satu desa (account_type=customer) dgn entity_code DESA-xxx.
func seedAccount(ctx context.Context, q *db.Queries, tenantID int64, name string) (int64, error) {
	code, err := q.GenerateEntityCode(ctx, tenantID, codes.EntityAccount)
	if err != nil {
		return 0, fmt.Errorf("kode desa: %w", err)
	}
	a, err := q.CreateAccount(ctx, db.CreateAccountParams{
		TenantID:    tenantID,
		EntityCode:  &code,
		VillageName: name,
		AccountType: "customer",
	})
	if err != nil {
		return 0, fmt.Errorf("buat desa %q: %w", name, err)
	}
	return a.ID, nil
}

// createActive membuat langganan Active dgn end_date tertentu (menentukan jendela
// dasbor Renewals: due bila ≥ hari ini, grace bila < hari ini).
func createActive(ctx context.Context, q *db.Queries, tenantID, accountID, planID int64, mrr string, endDate time.Time) (db.Subscription, error) {
	return createSub(ctx, q, tenantID, accountID, planID, "Active", mrr, &endDate)
}

// createChurned membuat langganan lalu men-churn-nya (isi tipe/alasan/MRR hilang).
func createChurned(ctx context.Context, q *db.Queries, tenantID, accountID, planID int64, churnType, reason, mrr string) error {
	s, err := createSub(ctx, q, tenantID, accountID, planID, "Churned", mrr, nil)
	if err != nil {
		return err
	}
	lost, err := num(mrr)
	if err != nil {
		return err
	}
	ct, rs := churnType, reason
	if err := q.ChurnSubscription(ctx, db.ChurnSubscriptionParams{
		Status:           "Churned",
		CancellationDate: pgtype.Date{Time: time.Now(), Valid: true},
		ChurnReason:      &rs,
		ChurnType:        &ct,
		LostValueMrr:     lost,
		ID:               s.ID,
	}); err != nil {
		return fmt.Errorf("churn langganan #%d: %w", s.ID, err)
	}
	return nil
}

// createSub adalah pembungkus CreateSubscription bersama alokasi entity_code SUB-xxx.
func createSub(ctx context.Context, q *db.Queries, tenantID, accountID, planID int64, status, mrr string, endDate *time.Time) (db.Subscription, error) {
	code, err := q.GenerateEntityCode(ctx, tenantID, codes.EntitySubscription)
	if err != nil {
		return db.Subscription{}, fmt.Errorf("kode langganan: %w", err)
	}
	mrrNum, err := num(mrr)
	if err != nil {
		return db.Subscription{}, err
	}
	arrNum, err := num(mrr) // ARR contoh = MRR (nilai demo, bukan MRR×12)
	if err != nil {
		return db.Subscription{}, err
	}
	var end pgtype.Date
	if endDate != nil {
		end = pgtype.Date{Time: *endDate, Valid: true}
	}
	s, err := q.CreateSubscription(ctx, db.CreateSubscriptionParams{
		TenantID:   tenantID,
		EntityCode: &code,
		AccountID:  accountID,
		PlanID:     planID,
		Status:     status,
		StartDate:  pgtype.Date{Time: time.Now().AddDate(-1, 0, 0), Valid: true},
		EndDate:    end,
		AutoRenew:  true,
		Mrr:        mrrNum,
		Arr:        arrNum,
	})
	if err != nil {
		return db.Subscription{}, fmt.Errorf("buat langganan (%s): %w", status, err)
	}
	return s, nil
}

// num mengubah string desimal → pgtype.Numeric (mengikuti pola test numFrom).
func num(s string) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		return n, fmt.Errorf("numeric %q: %w", s, err)
	}
	return n, nil
}

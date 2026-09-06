// Command seeddemo mengisi DB dev dengan data contoh MENYELURUH — seluruh 19
// tabel Modul 2-8 CRM (Accounts/Contacts/Leads/Deals/Quotes/Subscriptions/
// Customer Success/Activities/Tickets/Engagements/dst) — agar dasbor & 4
// halaman Reports langsung terlihat berisi dgn proporsi wajar. BUKAN untuk
// produksi.
//
// Jalankan dari worktree yang punya .env (mis. folder utama):
//
//	go run ./cmd/seeddemo                 # ke workspace primer
//	go run ./cmd/seeddemo -slug my-space  # ke workspace tertentu
//
// Idempotent-friendly SEBAGIAN: tiap run memakai tag waktu unik pada kode/nama
// entitas ber-tag (entity_code, plan_code, email, dst) sehingga rerun tak
// bentrok unique index. PENGECUALIAN sejak BL-66: village_code kini kode
// Kemendagri ASLI dari kolam Desa tetap (regions level 4), jadi rerun ke tenant
// SAMA akan bentrok idx_accounts_code — hapus dulu accounts lama sebelum seed
// ulang (keputusan "hapus saja, seed ulang"). Data ini bertanda seed — aman
// dihapus manual kapan saja.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"go_starter/internal/config"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	slug := flag.String("slug", "", "slug workspace tujuan (kosong = workspace primer)")
	flag.Parse()

	if err := run(*slug); err != nil {
		log.Fatalf("seeddemo: %v", err)
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
	fmt.Fprintf(os.Stderr, "seeddemo: menargetkan workspace %q (tenant #%d)\n", tenantName, tenantID)

	tag := time.Now().Format("0102-150405") // pembeda run: MMDD-HHMMSS (test-only)
	var stats seedStats
	if err := db.WithTenant(ctx, pool, tenantID, func(q *db.Queries) error {
		s, err := seedInto(ctx, q, tenantID, tag)
		stats = s
		return err
	}); err != nil {
		return err
	}
	stats.print(os.Stderr)
	return nil
}

// resolveTenant memilih tenant tujuan: by-slug bila diberi, selain itu
// workspace primer (rumah aplikasi). Dibaca via WithSuper (tenants di luar
// RLS). owner (Amalia — satu-satunya anggota ber-business_role saat ini)
// diambil sekalian via ListMembersByTenant supaya kolom ownership (F3) bisa
// diisi user nyata, bukan hardcode ID.
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

// primaryOwner mengembalikan user_id anggota pertama workspace (Amalia di
// tenant primer dev) — dipakai sebagai ownership default utk sebagian besar
// record; sisanya sengaja NULL (keputusan disetujui user: campuran, bukan
// semua-terisi atau semua-kosong).
func primaryOwner(ctx context.Context, q *db.Queries, tenantID int64) (*int64, error) {
	members, err := q.ListMembersByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("cari owner: %w", err)
	}
	if len(members) == 0 {
		return nil, nil // workspace kosong anggota — semua ownership NULL, tetap valid.
	}
	return &members[0].UserID, nil
}

// seedInto mengorkestrasi seluruh domain sesuai urutan FK, mengumpulkan
// ringkasan jumlah baris utk dicetak di akhir.
func seedInto(ctx context.Context, q *db.Queries, tenantID int64, tag string) (seedStats, error) {
	rng := newRNG(tag)
	var stats seedStats

	owner, err := primaryOwner(ctx, q, tenantID)
	if err != nil {
		return stats, err
	}

	districts, err := pickDistricts(ctx, q)
	if err != nil {
		return stats, fmt.Errorf("pilih kecamatan: %w", err)
	}
	// BL-66: kolam Desa/Kelurahan REAL (regions level 4) dibagikan DISTINCT ke
	// accounts + leads (+ account hasil konversi) — village_code = kode
	// Kemendagri asli, bukan segmen ke-4 fiktif.
	pool, err := newVillagePool(ctx, q, rng, districts)
	if err != nil {
		return stats, fmt.Errorf("kolam desa: %w", err)
	}

	masters, err := seedMasters(ctx, q, tenantID, tag, owner)
	if err != nil {
		return stats, fmt.Errorf("masters: %w", err)
	}
	stats.Plans = len(masters.plans)
	stats.SLAPolicies = len(masters.slaPolicies)
	stats.Playbooks = masters.playbookCount
	stats.KBArticles = masters.kbArticleCount

	accounts, err := seedAccounts(ctx, q, tenantID, tag, rng, owner, pool)
	if err != nil {
		return stats, fmt.Errorf("accounts: %w", err)
	}
	stats.Accounts = len(accounts)

	contacts, err := seedContacts(ctx, q, tenantID, rng, owner, accounts)
	if err != nil {
		return stats, fmt.Errorf("contacts: %w", err)
	}
	stats.Contacts = len(contacts)

	leadResult, err := seedLeads(ctx, q, tenantID, tag, rng, owner, pool)
	if err != nil {
		return stats, fmt.Errorf("leads: %w", err)
	}
	stats.Leads = leadResult.total
	accounts = append(accounts, leadResult.convertedAccounts...)

	deals, err := seedDeals(ctx, q, tenantID, tag, rng, owner, accounts, masters.plans)
	if err != nil {
		return stats, fmt.Errorf("deals: %w", err)
	}
	stats.Deals = len(deals)

	quoteCount, itemCount, err := seedQuotes(ctx, q, tenantID, tag, rng, owner, accounts, deals, masters.plans)
	if err != nil {
		return stats, fmt.Errorf("quotes: %w", err)
	}
	stats.Quotes = quoteCount
	stats.QuoteItems = itemCount

	subs, err := seedSubscriptions(ctx, q, tenantID, tag, rng, owner, accounts, masters.plans)
	if err != nil {
		return stats, fmt.Errorf("subscriptions: %w", err)
	}
	stats.Subscriptions = len(subs)

	csCount, err := seedCustomerSuccess(ctx, q, tenantID, rng, accounts)
	if err != nil {
		return stats, fmt.Errorf("customer_success: %w", err)
	}
	stats.CustomerSuccess = csCount

	// Tickets DULU, baru Activities — activities.go butuh ID tiket nyata sbg
	// target_id polimorfik utk target_type='ticket' (bukan placeholder).
	tickets, err := seedTickets(ctx, q, tenantID, rng, owner, accounts, masters.slaPolicies)
	if err != nil {
		return stats, fmt.Errorf("tickets: %w", err)
	}
	stats.Tickets = len(tickets)

	actCount, err := seedActivities(ctx, q, tenantID, rng, owner, accounts, contacts, deals, subs, tickets)
	if err != nil {
		return stats, fmt.Errorf("activities: %w", err)
	}
	stats.Activities = actCount

	eng, err := seedEngagementsBundle(ctx, q, tenantID, rng, owner, accounts)
	if err != nil {
		return stats, fmt.Errorf("engagements bundle: %w", err)
	}
	stats.Engagements = eng.engagements
	stats.SuccessPlans = eng.successPlans
	stats.ImplTasks = eng.implTasks
	stats.Trainings = eng.trainings

	return stats, nil
}

// seedStats = ringkasan jumlah baris per tabel, dicetak ke stderr di akhir
// (pola sama cmd/seedsubs) supaya operator langsung tahu apa yang ter-insert.
type seedStats struct {
	Plans, SLAPolicies, Playbooks, KBArticles            int
	Accounts, Contacts, Leads, Deals, Quotes, QuoteItems int
	Subscriptions, CustomerSuccess, Activities, Tickets  int
	Engagements, SuccessPlans, ImplTasks, Trainings      int
}

func (s seedStats) print(w *os.File) {
	fmt.Fprintf(w, "seeddemo: selesai —\n")
	fmt.Fprintf(w, "  masters:  %d plans, %d sla_policies, %d playbooks, %d kb_articles\n",
		s.Plans, s.SLAPolicies, s.Playbooks, s.KBArticles)
	fmt.Fprintf(w, "  sales:    %d accounts, %d contacts, %d leads, %d deals, %d quotes (%d items)\n",
		s.Accounts, s.Contacts, s.Leads, s.Deals, s.Quotes, s.QuoteItems)
	fmt.Fprintf(w, "  subs+cs:  %d subscriptions, %d customer_success, %d activities, %d tickets\n",
		s.Subscriptions, s.CustomerSuccess, s.Activities, s.Tickets)
	fmt.Fprintf(w, "  engage:   %d engagements, %d success_plans, %d cs_impl_tasks, %d cs_trainings\n",
		s.Engagements, s.SuccessPlans, s.ImplTasks, s.Trainings)
}

package main

import (
	"context"
	"fmt"

	"go_starter/internal/db"
)

// purge.go — implementasi `seeddemo -reset` (BL-79): hapus SEMUA data demo CRM
// milik satu tenant sebelum isi ulang, agar rerun ke tenant yang sama tak
// bentrok unique index (khususnya village_code Kemendagri yang kini kode ASLI,
// bukan ber-tag — lihat catatan main.go). Yang DIHAPUS hanya 18 tabel domain
// CRM; tenant, memberships, users, dan master regions SENGAJA dibiarkan utuh
// supaya workspace + kolam desa tetap ada untuk seed berikutnya.
//
// Dijalankan di dalam db.WithTenant → RLS mengikat (app_rw), tiap DELETE juga
// eksplisit `WHERE tenant_id = $1`, jadi mustahil menyentuh workspace lain.

// purgeStep = satu langkah hapus bernama, supaya galat melaporkan tabel mana
// yang gagal alih-alih error pgx telanjang.
type purgeStep struct {
	table string
	del   func(context.Context, int64) error
}

// purgeSeedData menghapus seluruh baris demo tenant dalam URUTAN anak→induk.
// Urutan ini WAJIB: cs_impl_tasks/cs_trainings/engagements/success_plans/tickets
// mereferensi accounts dgn ON DELETE RESTRICT — menghapus accounts lebih dulu
// akan ditolak selagi anak-anaknya ada. Mengembalikan jumlah tabel yang diproses
// (bukan jumlah baris; DELETE :exec tak melaporkannya) untuk ringkasan operator.
func purgeSeedData(ctx context.Context, q *db.Queries, tenantID int64) (int, error) {
	steps := []purgeStep{
		{"activities", q.PurgeSeedActivities},
		{"quote_items", q.PurgeSeedQuoteItems},
		{"quotes", q.PurgeSeedQuotes},
		{"cs_impl_tasks", q.PurgeSeedCSImplTasks},
		{"cs_trainings", q.PurgeSeedCSTrainings},
		{"engagements", q.PurgeSeedEngagements},
		{"success_plans", q.PurgeSeedSuccessPlans},
		{"customer_success", q.PurgeSeedCustomerSuccess},
		{"subscriptions", q.PurgeSeedSubscriptions},
		{"tickets", q.PurgeSeedTickets},
		{"deals", q.PurgeSeedDeals},
		{"contacts", q.PurgeSeedContacts},
		{"leads", q.PurgeSeedLeads},
		{"accounts", q.PurgeSeedAccounts},
		{"kb_articles", q.PurgeSeedKBArticles},
		{"playbooks", q.PurgeSeedPlaybooks},
		{"sla_policies", q.PurgeSeedSLAPolicies},
		{"plans", q.PurgeSeedPlans},
	}
	for _, s := range steps {
		if err := s.del(ctx, tenantID); err != nil {
			return 0, fmt.Errorf("purge %s: %w", s.table, err)
		}
	}
	return len(steps), nil
}

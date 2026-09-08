-- seeddemo_purge.sql — DELETE per-tabel ber-tenant untuk `seeddemo -reset`
-- (BL-79). Bukan jalur produksi: dipakai HANYA oleh cmd/seeddemo untuk
-- membersihkan data demo sebelum isi ulang, dijalankan di dalam db.WithTenant
-- (RLS mengikat, app_rw punya hak DELETE). Preseden pola Purge* di package db:
-- PurgeTenant (tenants.sql) & PurgeAuditLogsBefore (audit.sql).
--
-- Semua saring `WHERE tenant_id = $1` — jadi tak pernah menyentuh workspace lain
-- bahkan seandainya RLS dilepas. Pemanggil (cmd/seeddemo/purge.go) mengurutkan
-- pemanggilan anak→induk sebab beberapa FK ber-ON DELETE RESTRICT (cs_impl_tasks,
-- cs_trainings, engagements, success_plans, tickets) menolak hapus induk selagi
-- anak ada; urutan di SATU query tak menjamin itu.

-- name: PurgeSeedActivities :exec
DELETE FROM activities WHERE tenant_id = $1;

-- name: PurgeSeedQuoteItems :exec
DELETE FROM quote_items WHERE tenant_id = $1;

-- name: PurgeSeedQuotes :exec
DELETE FROM quotes WHERE tenant_id = $1;

-- name: PurgeSeedCSImplTasks :exec
DELETE FROM cs_impl_tasks WHERE tenant_id = $1;

-- name: PurgeSeedCSTrainings :exec
DELETE FROM cs_trainings WHERE tenant_id = $1;

-- name: PurgeSeedEngagements :exec
DELETE FROM engagements WHERE tenant_id = $1;

-- name: PurgeSeedSuccessPlans :exec
DELETE FROM success_plans WHERE tenant_id = $1;

-- name: PurgeSeedCustomerSuccess :exec
DELETE FROM customer_success WHERE tenant_id = $1;

-- name: PurgeSeedSubscriptions :exec
DELETE FROM subscriptions WHERE tenant_id = $1;

-- name: PurgeSeedTickets :exec
DELETE FROM tickets WHERE tenant_id = $1;

-- name: PurgeSeedDeals :exec
DELETE FROM deals WHERE tenant_id = $1;

-- name: PurgeSeedContacts :exec
DELETE FROM contacts WHERE tenant_id = $1;

-- name: PurgeSeedLeads :exec
DELETE FROM leads WHERE tenant_id = $1;

-- name: PurgeSeedAccounts :exec
DELETE FROM accounts WHERE tenant_id = $1;

-- name: PurgeSeedKBArticles :exec
DELETE FROM kb_articles WHERE tenant_id = $1;

-- name: PurgeSeedPlaybooks :exec
DELETE FROM playbooks WHERE tenant_id = $1;

-- name: PurgeSeedSLAPolicies :exec
DELETE FROM sla_policies WHERE tenant_id = $1;

-- name: PurgeSeedPlans :exec
DELETE FROM plans WHERE tenant_id = $1;

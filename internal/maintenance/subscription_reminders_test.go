package maintenance

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// subscription_reminders_test.go — BL-158: pengingat terjadwal H-30/H-14/H-7/
// hari-H. Yang paling penting dibuktikan di sini BUKAN "notifikasi terkirim"
// (itu trivial) melainkan (a) ambang PERSIS 30/14/7/0 — bukan rentang, (b) anti-
// dobel via last_reminder_days_sent bertahan lintas SIKLUS (bukan cuma dalam
// satu proses), dan (c) pengecualian (owner NULL, status non-aktif, di luar
// ambang) benar-benar mengecualikan, bukan cuma kebetulan tak match hari ini.

// reminderPool mengosongkan seluruh rantai FK yang disentuh test ini. Tabel
// milik paket lain (audit_logs) TIDAK disentuh — cukup rantai subscription.
func reminderPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if pkgPool == nil {
		t.Skip("TEST_DATABASE_URL tidak di-set; lewati test yang butuh DB")
	}
	if _, err := pkgPool.Exec(t.Context(),
		"TRUNCATE notifications, subscriptions, plans, accounts, memberships, users, tenants RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pkgPool
}

// seedReminderTenant menyiapkan satu tenant + satu user (calon subscription_owner).
func seedReminderTenant(t *testing.T, pool *pgxpool.Pool, slug string) (tenantID, userID int64) {
	t.Helper()
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO tenants (name, slug, status) VALUES ($1, $1, 'active') RETURNING id`,
		slug).Scan(&tenantID); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO users (email) VALUES ($1) RETURNING id`,
		slug+"@example.test").Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return tenantID, userID
}

// seedReminderAccount + seedReminderPlan menyediakan FK wajib (account_id,
// plan_id) subscriptions — isinya sendiri tak relevan bagi test ini.
func seedReminderAccount(t *testing.T, pool *pgxpool.Pool, tenantID int64, villageName string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO accounts (tenant_id, village_name, account_type) VALUES ($1,$2,'customer') RETURNING id`,
		tenantID, villageName).Scan(&id); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	return id
}

func seedReminderPlan(t *testing.T, pool *pgxpool.Pool, tenantID int64, code string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO plans (tenant_id, plan_name, plan_code, plan_category) VALUES ($1,'Paket Inti',$2,'Core') RETURNING id`,
		tenantID, code).Scan(&id); err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	return id
}

// subSeed = parameter subscription yang RELEVAN bagi task ini; sisanya (mrr,
// billing_cycle, dst) tak dipakai kondisi query jadi diabaikan.
type subSeed struct {
	accountID int64
	planID    int64
	owner     *int64 // NULL → dikecualikan (keputusan user: tanpa fallback)
	status    string
	endInDays int // end_date = today + endInDays; nil pakai endDate langsung
	lastSent  *int32
}

func seedReminderSub(t *testing.T, pool *pgxpool.Pool, tenantID int64, s subSeed) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO subscriptions
		     (tenant_id, account_id, plan_id, subscription_owner, status, end_date, last_reminder_days_sent)
		 VALUES ($1,$2,$3,$4,$5, CURRENT_DATE + $6::int, $7)
		 RETURNING id`,
		tenantID, s.accountID, s.planID, s.owner, s.status, s.endInDays, s.lastSent).Scan(&id); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return id
}

func countReminderNotifs(t *testing.T, pool *pgxpool.Pool, userID int64) int64 {
	t.Helper()
	var n int64
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM notifications WHERE user_id=$1 AND kind='subscription.renewal.reminder'`,
		userID).Scan(&n); err != nil {
		t.Fatalf("hitung notifikasi: %v", err)
	}
	return n
}

func lastReminderSent(t *testing.T, pool *pgxpool.Pool, subID int64) *int32 {
	t.Helper()
	var v *int32
	if err := pool.QueryRow(t.Context(),
		`SELECT last_reminder_days_sent FROM subscriptions WHERE id=$1`, subID).Scan(&v); err != nil {
		t.Fatalf("baca last_reminder_days_sent: %v", err)
	}
	return v
}

// TestSubscriptionRenewalReminders_AmbangTepat: keempat ambang (30/14/7/0)
// masing-masing memicu TEPAT satu notifikasi dan menandai kolom anti-dobel.
func TestSubscriptionRenewalReminders_AmbangTepat(t *testing.T) {
	pool := reminderPool(t)
	tenantID, userID := seedReminderTenant(t, pool, "ambang")
	accID := seedReminderAccount(t, pool, tenantID, "Desa Ambang")
	planID := seedReminderPlan(t, pool, tenantID, "PLN-AMB")

	for _, days := range []int{30, 14, 7, 0} {
		seedReminderSub(t, pool, tenantID, subSeed{
			accountID: accID, planID: planID, owner: &userID,
			status: "Active", endInDays: days,
		})
	}

	task := SubscriptionRenewalReminders(pool, time.UTC, quietLog())
	n, err := task(t.Context())
	if err != nil {
		t.Fatalf("task: %v", err)
	}
	if n != 4 {
		t.Fatalf("dikirim = %d, mau 4 (satu per ambang)", n)
	}
	if got := countReminderNotifs(t, pool, userID); got != 4 {
		t.Fatalf("notifikasi tersimpan = %d, mau 4", got)
	}
}

// TestSubscriptionRenewalReminders_AntiDobel: siklus KEDUA pada hari yang sama
// tidak boleh mengirim notifikasi kedua untuk ambang yang sudah tercatat.
func TestSubscriptionRenewalReminders_AntiDobel(t *testing.T) {
	pool := reminderPool(t)
	tenantID, userID := seedReminderTenant(t, pool, "dobel")
	accID := seedReminderAccount(t, pool, tenantID, "Desa Dobel")
	planID := seedReminderPlan(t, pool, tenantID, "PLN-DBL")
	subID := seedReminderSub(t, pool, tenantID, subSeed{
		accountID: accID, planID: planID, owner: &userID,
		status: "Active", endInDays: 7,
	})

	task := SubscriptionRenewalReminders(pool, time.UTC, quietLog())
	if _, err := task(t.Context()); err != nil {
		t.Fatalf("siklus 1: %v", err)
	}
	if _, err := task(t.Context()); err != nil {
		t.Fatalf("siklus 2: %v", err)
	}

	if got := countReminderNotifs(t, pool, userID); got != 1 {
		t.Fatalf("notifikasi setelah 2 siklus = %d, mau 1 (anti-dobel)", got)
	}
	sent := lastReminderSent(t, pool, subID)
	if sent == nil || *sent != 7 {
		t.Fatalf("last_reminder_days_sent = %v, mau 7", sent)
	}
}

// TestSubscriptionRenewalReminders_Pengecualian: owner NULL, status non-aktif,
// dan end_date di luar 4 ambang HARUS dikecualikan — bukan kebetulan tak match.
func TestSubscriptionRenewalReminders_Pengecualian(t *testing.T) {
	pool := reminderPool(t)
	tenantID, userID := seedReminderTenant(t, pool, "kecuali")
	accID := seedReminderAccount(t, pool, tenantID, "Desa Kecuali")
	planID := seedReminderPlan(t, pool, tenantID, "PLN-KEC")

	// Owner NULL, tapi end_date pas ambang 7 hari → tetap dikecualikan.
	seedReminderSub(t, pool, tenantID, subSeed{
		accountID: accID, planID: planID, owner: nil,
		status: "Active", endInDays: 7,
	})
	// Status Churned, ambang 7 hari → dikecualikan (bukan langganan berjalan).
	seedReminderSub(t, pool, tenantID, subSeed{
		accountID: accID, planID: planID, owner: &userID,
		status: "Churned", endInDays: 7,
	})
	// end_date 15 hari lagi — di luar 30/14/7/0 → dikecualikan.
	seedReminderSub(t, pool, tenantID, subSeed{
		accountID: accID, planID: planID, owner: &userID,
		status: "Active", endInDays: 15,
	})

	task := SubscriptionRenewalReminders(pool, time.UTC, quietLog())
	n, err := task(t.Context())
	if err != nil {
		t.Fatalf("task: %v", err)
	}
	if n != 0 {
		t.Fatalf("dikirim = %d, mau 0 (semua baris seharusnya dikecualikan)", n)
	}
	if got := countReminderNotifs(t, pool, userID); got != 0 {
		t.Fatalf("notifikasi tersimpan = %d, mau 0", got)
	}
}

// TestSubscriptionRenewalReminders_AmbangBaruSetelahLama: last_reminder_days_sent
// yang menyimpan ambang LAMA (mis. 30) tak boleh menghalangi ambang BARU (14)
// yang berbeda nilainya — IS DISTINCT FROM membandingkan nilai, bukan "sudah
// pernah dikirim sama sekali".
func TestSubscriptionRenewalReminders_AmbangBaruSetelahLama(t *testing.T) {
	pool := reminderPool(t)
	tenantID, userID := seedReminderTenant(t, pool, "lanjut")
	accID := seedReminderAccount(t, pool, tenantID, "Desa Lanjut")
	planID := seedReminderPlan(t, pool, tenantID, "PLN-LJT")
	old := int32(30)
	subID := seedReminderSub(t, pool, tenantID, subSeed{
		accountID: accID, planID: planID, owner: &userID,
		status: "Active", endInDays: 14, lastSent: &old,
	})

	task := SubscriptionRenewalReminders(pool, time.UTC, quietLog())
	n, err := task(t.Context())
	if err != nil {
		t.Fatalf("task: %v", err)
	}
	if n != 1 {
		t.Fatalf("dikirim = %d, mau 1 (ambang 14 belum pernah dikirim)", n)
	}
	sent := lastReminderSent(t, pool, subID)
	if sent == nil || *sent != 14 {
		t.Fatalf("last_reminder_days_sent = %v, mau 14", sent)
	}
}

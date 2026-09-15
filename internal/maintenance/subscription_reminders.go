package maintenance

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// subscription_reminders.go — BL-158: notifikasi terjadwal saat subscription
// mendekati/mencapai end_date pada ambang PERSIS H-30/H-14/H-7/hari-H (bukan
// rentang, agar tiap ambang trigger SATU kali per siklus — cegah notif
// beruntun). Dipisah dari tasks.go (housekeeping murni/DELETE) karena file ini
// menyentuh domain notifikasi dengan payload sendiri.

// reminderKind = kind notifikasi (harus ada di CHECK notifications_kind_check,
// diperluas migrasi 00047).
const reminderKind = "subscription.renewal.reminder"

// reminderPayload = bentuk payload JSONB. TAG JSON HARUS SAMA dgn notifPayload
// (internal/handler/notifications_service.go) — paket ini SENGAJA tak mengimpor
// handler (menghindari coupling lintas layer maintenance→handler); kontrak yang
// disamakan hanyalah bentuk JSON yang dibaca notifText() saat render.
type reminderPayload struct {
	EntityCode  string `json:"entity_code,omitempty"`
	VillageName string `json:"village_name,omitempty"`
	DaysLeft    *int   `json:"days_left,omitempty"`
}

// SubscriptionRenewalReminders mengirim notifikasi ke subscription_owner saat
// sisa hari ke end_date TEPAT 30/14/7/0 (BL-158, sumber: user 12 Sep — "terapkan
// notifikasi <=30 hari, 14<=hari, 7<=hari dan hari h"). Penerima = HANYA
// subscription_owner (keputusan user: tanpa fallback ke tenant admin/owner bila
// kosong — subscription tanpa owner dikecualikan oleh query, bukan error).
// Anti-dobel = kolom subscriptions.last_reminder_days_sent (migrasi 00047):
// sekali ambang X terkirim, ambang X yang sama tak terkirim ulang walau task
// jalan berkali-kali sehari atau proses restart.
//
// loc dipakai menghitung "hari ini" di zona waktu app (cfg.Location()) — bukan
// AT TIME ZONE di SQL (gotcha #14, emit interface{} pada SELECT list sqlc).
func SubscriptionRenewalReminders(pool *pgxpool.Pool, loc *time.Location, log *slog.Logger) func(context.Context) (int64, error) {
	return func(ctx context.Context) (int64, error) {
		release, ok := tryLock(ctx, pool)
		if !ok {
			return 0, nil // instance lain sedang mengerjakannya
		}
		defer release()

		now := time.Now().In(loc)
		today := pgtype.Date{
			Time:  time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
			Valid: true,
		}

		var n int64
		err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
			due, e := q.ListSubscriptionsDueForReminder(ctx, today)
			if e != nil {
				return e
			}
			for _, s := range due {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				// SAVEPOINT per baris: satu subscription gagal (mis. FK longgar,
				// owner terhapus di antara SELECT & INSERT) tak boleh membatalkan
				// notifikasi+penandaan subscription lain dalam siklus yang sama.
				serr := db.WithSavepoint(ctx, q, func(q *db.Queries) error {
					return sendOneReminder(ctx, q, s)
				})
				if serr != nil {
					// ID saja, tanpa nama desa/owner — jejak pemeliharaan bukan
					// tempat PII (Rule 12).
					log.Error("maintenance: reminder gagal", "subscriptionID", s.ID, "err", serr)
					continue
				}
				n++
			}
			return nil
		})
		return n, err
	}
}

// sendOneReminder menulis notifikasi + menandai ambang terkirim utk SATU baris.
// Dipisah dari loop agar defer/scope savepoint di atas tetap jelas.
func sendOneReminder(ctx context.Context, q *db.Queries, s db.ListSubscriptionsDueForReminderRow) error {
	days := int(s.DaysLeft)
	entityCode := ""
	if s.EntityCode != nil {
		entityCode = *s.EntityCode
	}
	payload := reminderPayload{
		EntityCode:  entityCode,
		VillageName: s.VillageName,
		DaysLeft:    &days,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var tid *int64
	if s.TenantID > 0 {
		tid = &s.TenantID
	}
	// s.SubscriptionOwner dijamin non-nil oleh query (WHERE subscription_owner
	// IS NOT NULL) — dereference aman.
	if _, err := q.CreateNotification(ctx, db.CreateNotificationParams{
		UserID: *s.SubscriptionOwner, TenantID: tid, Kind: reminderKind, Payload: raw,
	}); err != nil {
		return err
	}

	return q.MarkSubscriptionReminderSent(ctx, db.MarkSubscriptionReminderSentParams{
		DaysLeft: &s.DaysLeft, ID: s.ID,
	})
}

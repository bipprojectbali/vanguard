package mcpserver

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"time"

	"go_starter/internal/activity"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// tools_activity.go — tool yang menjawab "apa yang terjadi & siapa yang aktif"
// dari jejak audit + presence. Merender lewat activity.Sentence (pure), meniru
// pemetaan handler/dev_logs_trail.go (yang unexported, jadi direplikasi ringkas).

func registerActivityTools(s *mcp.Server, d *deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "activity_trail",
		Description: "Jejak aktivitas terbaru (audit_logs) sebagai kalimat yang terbaca: siapa melakukan apa, kapan. Rentang: day|week|month. Read-only.",
	}, d.activityTrail)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "activity_kpis",
		Description: "Ringkasan kehadiran (presence) pada rentang: jumlah user aktif & total aktivitas. Rentang: day|week|month. Read-only.",
	}, d.activityKPIs)
}

// rangeInput dipakai dua tool. Default "day" bila kosong/tak dikenal
// (activity.ParseRange yang memutuskan).
type rangeInput struct {
	Range string `json:"range,omitempty" jsonschema:"rentang waktu: day, week, atau month (default day)"`
}

// window menurunkan batas [from,to) UTC dari rentang + zona aplikasi. Dipakai
// kedua tool agar keduanya menafsirkan "hari ini" dengan cara yang sama.
func (d *deps) window(in rangeInput) (pgtype.Timestamptz, pgtype.Timestamptz) {
	rng := activity.ParseRange(in.Range)
	loc := time.UTC
	if l, err := time.LoadLocation(d.cfg.AppTimezone); err == nil {
		loc = l
	}
	from, to := rng.Window(time.Now(), loc)
	return pgtype.Timestamptz{Time: from, Valid: true}, pgtype.Timestamptz{Time: to, Valid: true}
}

// --- activity_trail ---

type trailOut struct {
	Range  string      `json:"range"`
	Events []trailItem `json:"events" jsonschema:"peristiwa terbaru lebih dulu"`
}

type trailItem struct {
	Sentence string `json:"sentence" jsonschema:"kalimat yang menjelaskan peristiwa"`
	Action   string `json:"action" jsonschema:"kode aksi mentah, mis. workspace.create"`
	When     string `json:"when" jsonschema:"waktu peristiwa (UTC)"`
}

// trailPageSize membatasi jumlah peristiwa per panggilan. Kecil dengan sengaja:
// tool ini untuk "lihat yang terbaru saat menyelidiki", bukan mengekspor seluruh
// riwayat — itu tugas paginasi di panel /dev/logs.
const trailPageSize = 30

func (d *deps) activityTrail(ctx context.Context, _ *mcp.CallToolRequest, in rangeInput) (*mcp.CallToolResult, trailOut, error) {
	ctx, cancel := d.ctxWith(ctx)
	defer cancel()
	from, to := d.window(in)
	// Events non-nil → JSON "events":[] saat rentang kosong, bukan null.
	out := trailOut{Range: string(activity.ParseRange(in.Range)), Events: []trailItem{}}

	err := db.WithSuper(ctx, d.pool, func(q *db.Queries) error {
		rows, err := q.ListActivityTrail(ctx, db.ListActivityTrailParams{
			FromAt: from, ToAt: to,
			// Halaman pertama: cursor = maksimum agar semua baris lolos syarat.
			// Sama dengan firstPageCursor() yang teruji di handler (math.MaxInt64),
			// bukan 1<<62 — konsisten dengan pola keyset yang sudah ada.
			CursorCreatedAt: pgtype.Timestamptz{Valid: true, InfinityModifier: pgtype.Infinity},
			CursorID:        math.MaxInt64,
			PageSize:        trailPageSize,
		})
		if err != nil {
			return err
		}
		for _, r := range rows {
			out.Events = append(out.Events, trailItem{
				Sentence: activity.Sentence(trailEvent(r)),
				Action:   r.Action,
				When:     r.CreatedAt.Time.UTC().Format(time.RFC3339),
			})
		}
		return nil
	})
	return nil, out, err
}

// trailEvent memetakan baris SQL → activity.Event. Direplikasi dari
// handler/dev_logs_trail.go (unexported di sana; logika kecil & stabil, tak
// sepadan diangkat ke paket bersama untuk PoC ini).
func trailEvent(a db.ListActivityTrailRow) activity.Event {
	var m struct {
		To, Role, Value, Reason, Method string
	}
	if len(a.Metadata) > 0 {
		_ = json.Unmarshal(a.Metadata, &m) // metadata rusak → kalimat tanpa detail, bukan baris hilang
	}
	role := m.To
	if role == "" {
		role = m.Role
	}
	return activity.Event{
		Action:     a.Action,
		TargetType: a.TargetType,
		Actor:      ident(a.ActorName, a.ActorEmail),
		TargetUser: ident(a.TargetUserName, a.TargetUserEmail),
		Workspace:  deref(a.TargetWorkspaceName),
		Role:       role,
		Value:      m.Value,
		Reason:     m.Reason,
		Method:     m.Method,
	}
}

// ident memilih penanda orang: nama bila ada, selainnya email. (Sama semangat
// dengan handler; email TAK disamarkan — pembaca panel ini operator platform.)
func ident(name, email *string) string {
	if name != nil && strings.TrimSpace(*name) != "" {
		return *name
	}
	return deref(email)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// --- activity_kpis ---

type kpiOut struct {
	Range       string `json:"range"`
	ActiveUsers int64  `json:"active_users" jsonschema:"user unik yang aktif pada rentang"`
	TotalHits   int64  `json:"total_hits" jsonschema:"total aktivitas pada rentang"`
}

func (d *deps) activityKPIs(ctx context.Context, _ *mcp.CallToolRequest, in rangeInput) (*mcp.CallToolResult, kpiOut, error) {
	ctx, cancel := d.ctxWith(ctx)
	defer cancel()
	from, to := d.window(in)
	out := kpiOut{Range: string(activity.ParseRange(in.Range))}

	err := db.WithSuper(ctx, d.pool, func(q *db.Queries) error {
		kpi, err := q.PresenceKPIs(ctx, db.PresenceKPIsParams{FromAt: from, ToAt: to})
		if err != nil {
			return err
		}
		out.ActiveUsers, out.TotalHits = kpi.ActiveUsers, kpi.TotalHits
		return nil
	})
	return nil, out, err
}

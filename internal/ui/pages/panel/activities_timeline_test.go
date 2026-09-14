package panel

import (
	"strings"
	"testing"
)

// activities_timeline_test.go — regresi: baris kartu timeline (dipakai detail
// Deal/Contact/Lead/Account) kini bisa diklik menuju halaman detail aktivitas
// (permintaan user 14 Sep — sebelumnya baris statis, tak ada tautan sama sekali).
// Baris SUMBER CS (BL-31, engagement) SENGAJA dikecualikan — id-nya id
// engagement, bukan id activity, jadi menautkannya ke /activities/{id} salah
// sasaran.

func TestActivityTimeline_RowLinksToActivityDetail(t *testing.T) {
	out := renderLeads(t, ActivityTimeline(ActivityTimelineView{
		Base:       "/w/acme",
		TargetType: "lead",
		TargetID:   103,
		Items: []ActivityTimelineItem{
			{ID: 55, Kind: "task", Subject: "asd", Date: "14 Sep 2026", Status: "Deferred", Owner: "Amalia Dwi Yustiani"},
		},
	}))
	if !strings.Contains(out, `href="/w/acme/activities/55"`) {
		t.Errorf("baris timeline harus tertaut ke /activities/55:\n%s", out)
	}
	if !strings.Contains(out, "asd") {
		t.Errorf("baris timeline harus tetap menampilkan subjek:\n%s", out)
	}
}

// TestActivityTimeline_CSRowNotLinked: baris terpadu Source="cs" (engagement,
// BL-31) tak boleh ditautkan — item.ID adalah id engagement, bukan id activity.
func TestActivityTimeline_CSRowNotLinked(t *testing.T) {
	out := renderLeads(t, ActivityTimeline(ActivityTimelineView{
		Base:       "/w/acme",
		TargetType: "account",
		TargetID:   7,
		Items: []ActivityTimelineItem{
			{ID: 9, Subject: "QBR triwulan", Date: "1 Sep 2026", Source: "cs",
				TypeLabel: "QBR", Status: "Selesai", StatusBadgeClass: "badge badge-success"},
		},
	}))
	if strings.Contains(out, `href="/w/acme/activities/9"`) {
		t.Errorf("baris CS (engagement) tak boleh tertaut ke /activities/{id}:\n%s", out)
	}
	if !strings.Contains(out, "QBR triwulan") {
		t.Errorf("baris CS harus tetap tampil:\n%s", out)
	}
}

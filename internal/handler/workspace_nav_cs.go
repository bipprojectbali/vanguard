package handler

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// workspace_nav_cs.go — grup nav Customer Success (wireframe 6, Modul 6).
// Selalu tampil (peta jalan produk terlihat, "nol menu hantu"): 10 anak
// URUT PERSIS wireframe (6.1 → 6.11), SLA Management (A1) & Playbooks (A2)
// berbackend — sisanya placeholder disabled sampai slice-nya sendiri
// mendarat (A3 KB Articles, B1 Health/Journey/Adoption, C1 Onboarding,
// C2 Success Plans, C3 Surveys, D1 Tickets).

// workspaceCSGroup merakit grup Customer Success (wireframe 6). canSLA =
// canViewSLAPolicies (izin SAMA dengan gerbang SLAPoliciesList). canPlaybooks
// = canViewPlaybooks (izin SAMA dengan gerbang PlaybooksList).
func workspaceCSGroup(slug string, canSLA, canPlaybooks bool) ui.NavItem {
	children := make([]ui.NavItem, 0, 11)
	children = append(children,
		ui.NavItem{Label: "Health Score", Icon: lucide.HeartPulse(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Journey / Onboarding", Icon: lucide.Milestone(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Success Plans", Icon: lucide.ClipboardList(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Product Adoption", Icon: lucide.ChartPie(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Engagements", Icon: lucide.MessageSquare(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Renewal Management", Icon: lucide.CalendarClock(html.Class("size-4")), Disabled: true},
	)
	// Playbooks → /playbooks (canViewPlaybooks, objek crm:playbooks).
	// Berbackend sejak slice A2; disabled bila tak berhak, tetap tampil agar
	// posisi modul di peta jalan terlihat. Posisi TETAP di urutan wireframe
	// (6.7, antara Renewal Management & Voice of Customer) — bukan ditambah
	// di akhir daftar.
	playbooks := ui.NavItem{Label: "Playbooks", Icon: lucide.BookOpen(html.Class("size-4"))}
	if canPlaybooks {
		playbooks.Href = wsPath(slug, "/playbooks")
	} else {
		playbooks.Disabled = true
	}
	children = append(children, playbooks)
	children = append(children,
		ui.NavItem{Label: "Voice of Customer", Icon: lucide.MessageCircle(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Tickets / Cases", Icon: lucide.Ticket(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Knowledge Base", Icon: lucide.BookMarked(html.Class("size-4")), Disabled: true},
	)
	// SLA Management → /sla-policies (canViewSLAPolicies, objek crm:sla).
	// Berbackend sejak slice A1; disabled bila tak berhak, tetap tampil agar
	// posisi modul di peta jalan terlihat.
	sla := ui.NavItem{Label: "SLA Management", Icon: lucide.Timer(html.Class("size-4"))}
	if canSLA {
		sla.Href = wsPath(slug, "/sla-policies")
	} else {
		sla.Disabled = true
	}
	children = append(children, sla)
	return ui.NavItem{
		Label: "Customer Success", Icon: lucide.Heart(html.Class("size-4")), Children: children,
	}
}

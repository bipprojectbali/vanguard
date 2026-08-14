package handler

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// workspace_nav_cs.go — grup nav Customer Success (wireframe 6, Modul 6).
// Selalu tampil (peta jalan produk terlihat, "nol menu hantu"): Health Score
// (C1), SLA Management (A1), Playbooks (A2), Knowledge Base (A3) & Tickets/Cases
// (B2) berbackend — sisanya placeholder disabled sampai slice-nya mendarat.

// workspaceCSGroup merakit grup Customer Success (wireframe 6). canSLA =
// canViewSLAPolicies (izin SAMA dengan gerbang SLAPoliciesList). canPlaybooks
// = canViewPlaybooks. canKB = canViewKBArticles. canTickets = canViewTickets
// (izin SAMA dengan gerbang TicketsList, objek crm:tickets read).
// canHealthScore = canViewHealthScore (izin SAMA dengan gerbang HealthScoreList,
// objek crm:health read). canEngagements = canViewEngagements (izin SAMA dengan
// gerbang EngagementsList, objek crm:engagements read). canRenewals =
// canViewCSRenewals (crm:renewal_mgmt read).
func workspaceCSGroup(slug string, canSLA, canPlaybooks, canKB, canTickets, canHealthScore, canEngagements, canRenewals bool) ui.NavItem {
	children := make([]ui.NavItem, 0, 9)
	// Health Score → /health-scores (canViewHealthScore, objek crm:health read).
	// Berbackend sejak slice C1; disabled bila tak berhak, tetap tampil agar
	// posisi modul di peta jalan terlihat.
	healthScore := ui.NavItem{Label: "Health Score", Icon: lucide.HeartPulse(html.Class("size-4"))}
	if canHealthScore {
		healthScore.Href = wsPath(slug, "/health-scores")
	} else {
		healthScore.Disabled = true
	}
	children = append(children, healthScore)
	children = append(children,
		ui.NavItem{Label: "Success Plans", Icon: lucide.ClipboardList(html.Class("size-4")), Disabled: true},
	)
	// Engagements → /engagements (canViewEngagements, objek crm:engagements read).
	// Berbackend sejak slice 6.5; disabled bila tak berhak, tetap tampil agar
	// posisi modul di peta jalan terlihat.
	engagements := ui.NavItem{Label: "Engagements", Icon: lucide.MessageSquare(html.Class("size-4"))}
	if canEngagements {
		engagements.Href = wsPath(slug, "/engagements")
	} else {
		engagements.Disabled = true
	}
	children = append(children, engagements)
	// Renewal Management → /renewal-management (canViewCSRenewals, crm:renewal_mgmt read).
	// Berbackend sejak slice 6.6; disabled bila tak berhak, tetap tampil.
	renewal := ui.NavItem{Label: "Renewal Management", Icon: lucide.CalendarClock(html.Class("size-4"))}
	if canRenewals {
		renewal.Href = wsPath(slug, "/renewal-management")
	} else {
		renewal.Disabled = true
	}
	children = append(children, renewal)
	// Playbooks → /playbooks (canViewPlaybooks, objek crm:playbooks).
	// Berbackend sejak slice A2; disabled bila tak berhak, tetap tampil agar
	// posisi modul di peta jalan terlihat. Posisi TETAP di urutan wireframe
	// (6.7, antara Renewal Management & Voice of Customer).
	playbooks := ui.NavItem{Label: "Playbooks", Icon: lucide.BookOpen(html.Class("size-4"))}
	if canPlaybooks {
		playbooks.Href = wsPath(slug, "/playbooks")
	} else {
		playbooks.Disabled = true
	}
	children = append(children, playbooks)
	children = append(children,
		ui.NavItem{Label: "Voice of Customer", Icon: lucide.MessageCircle(html.Class("size-4")), Disabled: true},
	)
	// Tickets / Cases → /tickets (canViewTickets, objek crm:tickets read).
	// Berbackend sejak slice B2; disabled bila tak berhak, tetap tampil agar
	// posisi modul di peta jalan terlihat. Posisi TETAP di urutan wireframe
	// (6.9, antara Voice of Customer & Knowledge Base).
	tickets := ui.NavItem{Label: "Tickets / Cases", Icon: lucide.Ticket(html.Class("size-4"))}
	if canTickets {
		tickets.Href = wsPath(slug, "/tickets")
	} else {
		tickets.Disabled = true
	}
	children = append(children, tickets)
	// Knowledge Base → /kb-articles (canViewKBArticles, objek crm:kb).
	// Berbackend sejak slice A3; disabled bila tak berhak, tetap tampil agar
	// posisi modul di peta jalan terlihat. Posisi TETAP di urutan wireframe
	// (6.10, antara Tickets/Cases & SLA Management).
	kb := ui.NavItem{Label: "Knowledge Base", Icon: lucide.BookMarked(html.Class("size-4"))}
	if canKB {
		kb.Href = wsPath(slug, "/kb-articles")
	} else {
		kb.Disabled = true
	}
	children = append(children, kb)
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

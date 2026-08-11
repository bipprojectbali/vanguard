package mcpserver

import (
	"context"

	"go_starter/internal/erd"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// tools_schema.go — tool introspeksi skema database.

func registerSchemaTools(s *mcp.Server, d *deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "db_schema",
		Description: "Skema database live sebagai diagram Mermaid erDiagram (tabel, kolom, relasi FK). Berguna melihat bentuk skema staging/prod tanpa membuka DB. Read-only.",
	}, d.dbSchema)
}

type schemaOut struct {
	Mermaid    string `json:"mermaid" jsonschema:"skema sebagai teks Mermaid erDiagram, siap dirender"`
	TableCount int    `json:"table_count" jsonschema:"jumlah tabel yang terdeteksi"`
}

func (d *deps) dbSchema(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, schemaOut, error) {
	ctx, cancel := d.ctxWith(ctx)
	defer cancel()
	// Introspect menerima pool langsung & membaca information_schema — read-only
	// murni, tak butuh tenant/scope. current_schema() sudah difilter di dalamnya.
	sch, err := erd.Introspect(ctx, d.pool)
	if err != nil {
		return nil, schemaOut{}, err
	}
	return nil, schemaOut{Mermaid: sch.Mermaid(), TableCount: len(sch.Tables)}, nil
}

package mcpserver

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// protocol_test.go — verifikasi lewat PROTOKOL MCP nyata (in-process), bukan
// memanggil handler langsung. Membuktikan server benar-benar bicara MCP:
// handshake, daftar tool, dan panggil tool balas hasil. InMemoryTransport
// menghubungkan client↔server dalam satu proses (tanpa subprocess/HTTP).

// expectedTools = kontrak yang dijaga: menambah/menghapus tool tanpa sengaja
// akan memerahkan test ini.
var expectedTools = []string{
	"runtime_health", "preflight", "migration_version", "platform_stats",
	"db_schema", "activity_trail", "activity_kpis",
}

// connect merakit server, menyambungkannya ke client via InMemoryTransport, dan
// mengembalikan sesi client + cleanup.
func connect(t *testing.T) (*mcp.ClientSession, context.Context) {
	t.Helper()
	d := testDeps(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	srv := build(d.pool, d.cfg, d.log)
	clientT, serverT := mcp.NewInMemoryTransports()

	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs, ctx
}

// TestProtocol_ListTools: handshake + tools/list mengembalikan tepat ketujuh
// tool yang diharapkan.
func TestProtocol_ListTools(t *testing.T) {
	cs, ctx := connect(t)
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := map[string]bool{}
	for _, tl := range res.Tools {
		got[tl.Name] = true
		if tl.Description == "" {
			t.Errorf("tool %q tanpa deskripsi — agent tak tahu kapan memakainya", tl.Name)
		}
	}
	if len(res.Tools) != len(expectedTools) {
		t.Errorf("jumlah tool = %d, want %d", len(res.Tools), len(expectedTools))
	}
	for _, name := range expectedTools {
		if !got[name] {
			t.Errorf("tool %q tak terdaftar", name)
		}
	}
}

// TestProtocol_CallRuntimeHealth: panggil satu tool lewat protokol penuh dan
// pastikan balasannya berisi (bukan error, ada content).
func TestProtocol_CallRuntimeHealth(t *testing.T) {
	cs, ctx := connect(t)
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "runtime_health"})
	if err != nil {
		t.Fatalf("CallTool runtime_health: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool membalas error: %+v", res.Content)
	}
	if len(res.Content) == 0 {
		t.Error("balasan tool kosong — harus ada hasil terstruktur")
	}
}

// TestProtocol_CallActivityTrail_WithArg: tool berargumen (range) dipanggil
// lewat protokol dengan Arguments — memastikan schema input tersambung.
func TestProtocol_CallActivityTrail_WithArg(t *testing.T) {
	cs, ctx := connect(t)
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "activity_trail",
		Arguments: map[string]any{"range": "week"},
	})
	if err != nil {
		t.Fatalf("CallTool activity_trail: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool membalas error: %+v", res.Content)
	}
}

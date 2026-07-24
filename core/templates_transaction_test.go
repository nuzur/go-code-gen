package core

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// These tests guard an invariant that is invisible at generation time and only
// shows up as wrong data at runtime: a module method that accepts Options must
// honor WithSQLTransaction when it touches the database.
//
// The generated read paths originally ignored it. A caller that wrote inside a
// transaction and then read back through the module got a connection from the
// pool instead of its own transaction, so the read observed the pre-write
// snapshot. That surfaced as stale echoes from the granular schema tools, and
// as an outright false error from the ops that create something — the new row
// was missing from the read, so the lookup failed even though the write had
// committed.
//
// TestTemplatesParse only parses templates, so it cannot catch this. These
// checks are textual on purpose: they hold for any new template without needing
// a full generation run.

const templatesDir = "templates"

func readTemplates(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		t.Fatalf("failed to read %s: %v", templatesDir, err)
	}
	out := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tmpl") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(templatesDir, e.Name()))
		if err != nil {
			t.Fatalf("failed to read %s: %v", e.Name(), err)
		}
		out[e.Name()] = string(b)
	}
	if len(out) == 0 {
		t.Fatalf("no templates found in %s", templatesDir)
	}
	return out
}

// A template that resolves Options and reaches the database must consult SQLTx.
func TestOptionAwareTemplatesHonorTransaction(t *testing.T) {
	for name, body := range readTemplates(t) {
		if !strings.Contains(body, "applyAllOptions(") {
			continue // does not accept Options
		}
		if !strings.Contains(body, "m.repository.Queries") && !strings.Contains(body, "m.repository.DB") {
			continue // does not touch the database
		}
		if !strings.Contains(body, "SQLTx") {
			t.Errorf("%s resolves Options and touches the database but never consults SQLTx; "+
				"a caller's WithSQLTransaction would be silently ignored", name)
		}
	}
}

// The read paths must never issue their query on the pooled handle: that is the
// exact bug above. They have to go through the transaction-aware querier.
func TestReadTemplatesDoNotQueryPooledHandle(t *testing.T) {
	pooledQuery := regexp.MustCompile(`m\.repository\.Queries\.(Fetch|List)`)
	pooledExec := regexp.MustCompile(`m\.repository\.DB\.QueryContext`)

	for _, name := range []string{"core_module_fetch.go.tmpl", "core_module_list.go.tmpl"} {
		body := readTemplates(t)[name]
		if body == "" {
			t.Fatalf("%s not found", name)
		}
		if m := pooledQuery.FindString(body); m != "" {
			t.Errorf("%s queries the pooled handle via %q; use the transaction-aware querier "+
				"so a read inside a caller's transaction sees that transaction's writes", name, m)
		}
		if m := pooledExec.FindString(body); m != "" {
			t.Errorf("%s queries the pooled handle via %q; use the transaction-aware querier", name, m)
		}
	}
}

// A transactional read must not be served from the shared cache, nor collapsed
// into another caller's in-flight query by singleflight — both would hand back a
// result fetched on a different connection, defeating the transaction.
func TestReadTemplatesBypassSharedStateInsideTransaction(t *testing.T) {
	templates := readTemplates(t)
	for _, name := range []string{"core_module_fetch.go.tmpl", "core_module_list.go.tmpl"} {
		body := templates[name]
		if body == "" {
			t.Fatalf("%s not found", name)
		}
		if !strings.Contains(body, "resolvedOpts.SkipCache || resolvedOpts.SQLTx != nil") {
			t.Errorf("%s must derive its bypass from both SkipCache and SQLTx", name)
		}
		// The pre-fix code gated cache and singleflight on SkipCache alone, which
		// let a transactional read hit shared state.
		if strings.Contains(body, "!resolvedOpts.SkipCache {") {
			t.Errorf("%s still gates shared state on SkipCache alone; it must use the "+
				"combined bypass so transactional reads skip the cache and singleflight", name)
		}
		if strings.Contains(body, "m.sg.Do(cacheKey, func()") {
			t.Errorf("%s calls singleflight unconditionally; it must be skipped when a "+
				"transaction is supplied", name)
		}
	}
}

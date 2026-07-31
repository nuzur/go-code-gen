package core

import (
	"strings"
	"testing"
)

// The per-entity cache was fully implemented — keys, a 30s TTL, a singleflight
// group, a WithSkipCache option threaded through every method — and could never
// serve a single hit, because the module accessors used a VALUE receiver: the
// lazy `i.<entity> = <entity>.New(...)` landed on a copy that was discarded when
// the method returned. Every one of the ~200 accessor calls in the generated
// transports therefore built a fresh module with a fresh, empty cache (and its
// own janitor goroutine).
//
// The fix builds every module once, in New. A pointer receiver alone would have
// made the assignment stick but left the lazy branch racing, since the accessors
// are called concurrently from request handlers.
func TestCoreAccessorsReturnAStableModule(t *testing.T) {
	body := readTemplates(t)["core_main.go.tmpl"]
	if body == "" {
		t.Fatal("core_main.go.tmpl not found")
	}

	if strings.Contains(body, "func (i Implementation)") {
		t.Error("core_main.go.tmpl declares a method on a VALUE receiver; an accessor that " +
			"initializes its module on a copy throws the module (and its cache) away")
	}
	if !strings.Contains(body, "func (i *Implementation) {{$entity.Identifier | ToCamelCase}}()") {
		t.Error("core_main.go.tmpl: the entity accessors must be declared on a pointer receiver")
	}
	if !strings.Contains(body, "impl.{{$entity.Identifier}} = {{$entity.Identifier}}.New(") {
		t.Error("core_main.go.tmpl: modules must be constructed in New, so every accessor call " +
			"returns the same instance (and therefore the same cache)")
	}
	if strings.Contains(body, "if i.{{$entity.Identifier}} == nil {") {
		t.Error("core_main.go.tmpl still initializes a module lazily inside an accessor; that " +
			"branch is a data race once the accessors share a pointer")
	}
}

// With a cache that finally holds entries, a write must invalidate it — else a
// read issued after a create/update is answered from a snapshot that predates
// the write, for up to the cache TTL. WithSkipCache on the read-back inside the
// create handlers covers only that one read.
func TestWriteTemplatesInvalidateTheCache(t *testing.T) {
	templates := readTemplates(t)
	for _, name := range []string{
		"core_module_upsert_insert.go.tmpl",
		"core_module_upsert_update.go.tmpl",
		"core_module_delete.go.tmpl",
	} {
		body := templates[name]
		if body == "" {
			t.Fatalf("%s not found", name)
		}
		if !strings.Contains(body, "m.cache.Flush()") {
			t.Errorf("%s writes to the database but never invalidates the module cache; reads "+
				"after this write would be served from a pre-write snapshot", name)
		}
	}
}

// The list cache key used to be a fmt of the whole request, which embeds the
// *filtering.Declarations POINTER — freshly allocated per request by
// <entity>Declarations(). No two keys could ever match, so the list cache only
// ever grew and the singleflight group never collapsed anything. The built query
// already encodes filter, ordering, projection and page.
func TestListCacheKeyIsStableAcrossRequests(t *testing.T) {
	body := readTemplates(t)["core_module_list.go.tmpl"]
	if body == "" {
		t.Fatal("core_module_list.go.tmpl not found")
	}
	if strings.Contains(body, `cacheKey := fmt.Sprintf("List{{.EntityName}}:%v", request)`) {
		t.Error("core_module_list.go.tmpl keys the cache on the formatted request, which contains " +
			"a per-request pointer; the key must be derived from the built query")
	}
	if !strings.Contains(body, `cacheKey := "List{{.EntityName}}:" + query`) {
		t.Error("core_module_list.go.tmpl: the list cache key must be derived from the built query")
	}
}

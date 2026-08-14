package repo

import (
	"fmt"
	"strings"

	projecttypes "github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/nuzur/sql-gen/tosql"
)

// verifySelectContract fails generation when this package's select resolver and
// sql-gen's disagree about which selects an entity gets.
//
// The two are independent implementations of the same rule: sql-gen names the
// sqlc queries, and for every select this package resolves the module layer mints
// a Fetch<Name> wrapper that calls queries.Fetch<Name>. When they drift, nothing
// here fails — the generated project fails later, in the app's own build, with
// something like `queries.FetchNotificationByUserUUIDAndStatus undefined`, which
// is both far from the cause and only reachable by deploying. Checking the two
// name sets here turns that into an immediate, local error.
//
// consumed must be the project version that was handed to GenerateSQL *after* it
// returned, not the raw one: GenerateSQL mutates it in place via
// EnsureUniqueFieldIndexes, which desugars every `unique: true` field into a
// synthesized UNIQUE index. project.New already normalizes the same way, so want
// contains a name for each of those; comparing against a pre-call value would
// report every unique-flagged field as a missing query — a false positive for
// names sql-gen does in fact emit.
//
// got being a strict superset of want is normal and not an error: sql-gen also
// emits the synthesized unique indexes and combined-index selects. Only the other
// direction — a name the module layer will call that sql-gen does not emit as a
// simple select — breaks the build.
//
// The ORDER BY variants are checked the same way. They used to be part of the
// tolerated superset because nothing called them; now that the fetch wrapper
// switches on the time fields, a drift in the time-field rule or in the camel
// casing of a column name is once again a name the module layer calls and sql-gen
// never emitted — so both sides expand their selects into the same
// <Name>OrderedBy<TimeField>ASC/DESC list before comparing.
func verifySelectContract(project *projecttypes.Project, consumed *nemgen.ProjectVersion, dbType db.DBType) error {
	consumedEntities := map[string]*nemgen.Entity{}
	if consumed != nil {
		for _, e := range consumed.Entities {
			if e != nil {
				consumedEntities[e.Uuid] = e
			}
		}
	}

	for _, e := range project.Entities() {
		if e.Type != nemgen.EntityType_ENTITY_TYPE_STANDALONE {
			continue
		}

		want := []string{}
		for _, s := range ResolveSelectStatements(project, e) {
			want = append(want, s.Name)
			if !s.SortSupported {
				continue
			}
			for _, tf := range s.TimeFields {
				want = append(want, orderedNames(s.Name, tf.Name)...)
			}
		}

		// select_indexed_combined.sql is not one of the files this generator
		// ships, so a combined select is not a query the module layer can call.
		got := []string{}
		if consumedEntity, found := consumedEntities[e.Uuid]; found {
			for _, s := range tosql.ResolveSelectStatements(consumedEntity, dbType) {
				if s.CombinedIndexes {
					continue
				}
				got = append(got, s.Name)
				if !s.SortSupported {
					continue
				}
				for _, tf := range s.TimeFields {
					got = append(got, orderedNames(s.Name, tf.NameTitle)...)
				}
			}
		}

		if missing := missingNames(want, got); len(missing) > 0 {
			parts := []string{}
			for _, name := range missing {
				parts = append(parts, fmt.Sprintf("module layer will call Fetch%s but sql-gen does not emit it as a simple select", name))
			}
			return fmt.Errorf("entity %q: %s", e.Identifier, strings.Join(parts, "; "))
		}

		// sqlc parses every query in a file into one Go method, so two queries
		// sharing a name abort `sqlc generate` and take the whole generated
		// repository with it.
		if dupes := duplicateNames(got); len(dupes) > 0 {
			return fmt.Errorf("entity %q: sql-gen emits duplicate query name %q; sqlc will reject it", e.Identifier, dupes[0])
		}
	}

	return nil
}

// orderedNames is the pair of ORDER BY query names a select gets for one time
// field, spelled exactly as select_indexed_simple_*.tmpl names them and as
// core_module_fetch.go.tmpl calls them.
func orderedNames(selectName, timeFieldName string) []string {
	return []string{
		fmt.Sprintf("%sOrderedBy%sASC", selectName, timeFieldName),
		fmt.Sprintf("%sOrderedBy%sDESC", selectName, timeFieldName),
	}
}

// missingNames returns the names in want that do not appear in got, in want's
// order, each reported once.
func missingNames(want, got []string) []string {
	have := make(map[string]bool, len(got))
	for _, n := range got {
		have[n] = true
	}
	missing := []string{}
	reported := map[string]bool{}
	for _, n := range want {
		if !have[n] && !reported[n] {
			missing = append(missing, n)
			reported[n] = true
		}
	}
	return missing
}

// duplicateNames returns the names that occur more than once, in the order of
// their second occurrence, each reported once.
func duplicateNames(names []string) []string {
	seen := map[string]bool{}
	reported := map[string]bool{}
	dupes := []string{}
	for _, n := range names {
		if seen[n] && !reported[n] {
			dupes = append(dupes, n)
			reported[n] = true
		}
		seen[n] = true
	}
	return dupes
}

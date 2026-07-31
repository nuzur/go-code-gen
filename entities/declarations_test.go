package entities

import (
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"
)

// An enum filter has to work over BOTH transports, and REST has no protobuf enum
// types at all (an app generated with --api rest has no proto package), so the
// accepted spellings and their numbers travel with the declarations instead.
//
// The numbers must match how the value is stored: the generated Go enum uses
// iota over the schema's static values, and the generated proto enum numbers
// them by the same index. The proto spelling must match what the proto enum
// generator emits, or a gRPC filter written from the .proto file resolves to
// nothing.
func TestEnumValueDeclarations(t *testing.T) {
	enum := &nemgen.Enum{
		Identifier: "process_method",
		StaticValues: []*nemgen.EnumValue{
			{Identifier: "invalid"},
			{Identifier: "washed"},
			{Identifier: "very_natural"},
		},
	}

	got := enumValueDeclarations(enum, "ProcessMethod")
	if len(got) != 3 {
		t.Fatalf("expected 3 values, got %d", len(got))
	}

	for i, want := range []struct {
		number    int
		protoName string
		names     []string
	}{
		{0, "PROCESS_METHOD_INVALID", []string{"invalid", "PROCESS_METHOD_INVALID"}},
		{1, "PROCESS_METHOD_WASHED", []string{"washed", "PROCESS_METHOD_WASHED"}},
		{2, "PROCESS_METHOD_VERY_NATURAL", []string{"very_natural", "PROCESS_METHOD_VERY_NATURAL"}},
	} {
		if got[i].Number != want.number {
			t.Errorf("value %d: number = %d, want %d", i, got[i].Number, want.number)
		}
		if got[i].ProtoName != want.protoName {
			t.Errorf("value %d: proto name = %q, want %q", i, got[i].ProtoName, want.protoName)
		}
		if len(got[i].Names) != len(want.names) {
			t.Fatalf("value %d: names = %v, want %v", i, got[i].Names, want.names)
		}
		for j, n := range want.names {
			if got[i].Names[j] != n {
				t.Errorf("value %d: names = %v, want %v", i, got[i].Names, want.names)
			}
		}
	}
}

// The spellings are emitted as map keys in a composite literal, so a value that
// spells the same way twice would not compile.
func TestEnumValueDeclarationsDedupeSpellings(t *testing.T) {
	enum := &nemgen.Enum{
		Identifier: "status",
		StaticValues: []*nemgen.EnumValue{
			{Identifier: "STATUS_ACTIVE"},
			{Identifier: "active"},
		},
	}

	seen := map[string]bool{}
	for _, v := range enumValueDeclarations(enum, "Status") {
		for _, n := range v.Names {
			if seen[n] {
				t.Errorf("spelling %q emitted twice; the generated map literal would not compile", n)
			}
			seen[n] = true
		}
	}
}

package bango

import (
	"encoding/json"
	"slices"
	"testing"
)

func schemaDoc(t *testing.T) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(Schema(), &doc); err != nil {
		t.Fatalf("the schema is not JSON: %v", err)
	}
	return doc
}

func enumIn(t *testing.T, doc map[string]any, name string) []string {
	t.Helper()
	defs, ok := doc["$defs"].(map[string]any)
	if !ok {
		t.Fatal("the schema has no $defs")
	}
	one, ok := defs[name].(map[string]any)
	if !ok {
		t.Fatalf("the schema has no $defs.%s", name)
	}
	raw, ok := one["enum"].([]any)
	if !ok {
		t.Fatalf("$defs.%s has no enum", name)
	}
	var out []string
	for _, value := range raw {
		out = append(out, value.(string))
	}
	slices.Sort(out)
	return out
}

func TestTheSchemaListsTheMarksTheRendererKnows(t *testing.T) {
	var want []string
	for mark := range unicodeMarks {
		want = append(want, string(mark))
	}
	slices.Sort(want)
	if got := enumIn(t, schemaDoc(t), "mark"); !slices.Equal(got, want) {
		t.Errorf("the schema and the renderer disagree about marks:\n schema %v\n code   %v", got, want)
	}
}

func TestTheSchemaListsTheKindsTheRendererKnows(t *testing.T) {
	want := []string{""}
	for _, kind := range []Kind{KindText, KindPath, KindCount, KindTime, KindRef} {
		want = append(want, string(kind))
	}
	slices.Sort(want)
	if got := enumIn(t, schemaDoc(t), "kind"); !slices.Equal(got, want) {
		t.Errorf("the schema and the renderer disagree about kinds:\n schema %v\n code   %v", got, want)
	}
}

func TestTheSchemaReservesTheKeysTheRendererReserves(t *testing.T) {
	doc := schemaDoc(t)
	defs := doc["$defs"].(map[string]any)
	action := defs["action"].(map[string]any)["properties"].(map[string]any)
	refused := action["key"].(map[string]any)["not"].(map[string]any)["enum"].([]any)
	var got []string
	for _, value := range refused {
		got = append(got, value.(string))
	}
	slices.Sort(got)
	if want := ReservedKeys(); !slices.Equal(got, want) {
		t.Errorf("the schema and the renderer disagree about reserved keys:\n schema %v\n code   %v", got, want)
	}
}

func TestTheSchemaNamesTheDocumentVersionTheRendererSpeaks(t *testing.T) {
	doc := schemaDoc(t)
	got := doc["properties"].(map[string]any)["bango"].(map[string]any)["const"]
	if int(got.(float64)) != Version {
		t.Errorf("the schema pins version %v, this renderer speaks %d", got, Version)
	}
}

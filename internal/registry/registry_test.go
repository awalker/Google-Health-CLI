package registry

import "testing"

func TestRegistryMatchesDocumentedSurface(t *testing.T) {
	if got, want := len(Types()), 34; got != want {
		t.Fatalf("Types() length = %d, want %d", got, want)
	}
	if got, want := len(RESTOperations()), 18; got != want {
		t.Fatalf("RESTOperations() length = %d, want %d", got, want)
	}
}

func TestNutritionDataTypes(t *testing.T) {
	tests := map[string]struct {
		endpoint        string
		filter          string
		recordType      string
		defaultTimePath string
		operations      []string
	}{
		"nutrition-log": {
			endpoint:   "nutrition-log",
			filter:     "nutrition_log",
			recordType: "Sample",
			operations: []string{"list", "get", "reconcile", "rollup", "dailyRollUp", "create", "update", "batchDelete"},
		},
		"food": {
			endpoint:   "food",
			filter:     "food",
			recordType: "Food",
			operations: []string{"list", "get"},
		},
		"food-measurement-unit": {
			endpoint:   "food-measurement-unit",
			filter:     "food_measurement_unit",
			recordType: "Food",
			operations: []string{"list", "get"},
		},
	}

	for name, want := range tests {
		dataType, ok := Lookup(name)
		if !ok {
			t.Fatalf("Lookup(%q) failed", name)
		}
		if dataType.EndpointName != want.endpoint {
			t.Fatalf("%s endpoint = %q, want %q", name, dataType.EndpointName, want.endpoint)
		}
		if dataType.FilterName != want.filter {
			t.Fatalf("%s filter = %q, want %q", name, dataType.FilterName, want.filter)
		}
		if dataType.RecordType != want.recordType {
			t.Fatalf("%s record type = %q, want %q", name, dataType.RecordType, want.recordType)
		}
		if dataType.Scope != "nutrition" {
			t.Fatalf("%s scope = %q, want nutrition", name, dataType.Scope)
		}
		if dataType.DefaultTimePath != want.defaultTimePath {
			t.Fatalf("%s time path = %q, want %q", name, dataType.DefaultTimePath, want.defaultTimePath)
		}
		for _, operation := range want.operations {
			if !HasOperation(dataType, operation) {
				t.Fatalf("%s missing operation %q", name, operation)
			}
		}
	}
}

func TestLookupAcceptsEndpointAndFilterNames(t *testing.T) {
	tests := []string{"heart-rate-variability", "heart_rate_variability", "Heart Rate Variability"}
	for _, test := range tests {
		dataType, ok := Lookup(test)
		if !ok {
			t.Fatalf("Lookup(%q) failed", test)
		}
		if dataType.EndpointName != "heart-rate-variability" {
			t.Fatalf("Lookup(%q) endpoint = %q", test, dataType.EndpointName)
		}
	}
}

func TestFilterFromRange(t *testing.T) {
	dataType, ok := Lookup("heart-rate")
	if !ok {
		t.Fatal("heart-rate missing")
	}
	got := FilterFromRange(dataType, "2026-05-08T00:00:00Z", "2026-05-09T00:00:00Z")
	want := `heart_rate.sample_time.physical_time >= "2026-05-08T00:00:00Z" AND heart_rate.sample_time.physical_time < "2026-05-09T00:00:00Z"`
	if got != want {
		t.Fatalf("FilterFromRange() = %q, want %q", got, want)
	}
}

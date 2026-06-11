package registry

import "testing"

func TestRegistryMatchesDocumentedSurface(t *testing.T) {
	if got, want := len(Types()), 31; got != want {
		t.Fatalf("Types() length = %d, want %d", got, want)
	}
	if got, want := len(RESTOperations()), 18; got != want {
		t.Fatalf("RESTOperations() length = %d, want %d", got, want)
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

func TestFilterFromRangeNonFilterable(t *testing.T) {
	nonFilterable := []string{"exercise", "sleep", "daily-resting-heart-rate"}
	for _, name := range nonFilterable {
		dt, ok := Lookup(name)
		if !ok {
			t.Fatalf("Lookup(%q) failed", name)
		}
		if dt.Filterable {
			t.Errorf("%s should not be Filterable", name)
		}
		got := FilterFromRange(dt, "2026-05-01T00:00:00Z", "2026-06-01T00:00:00Z")
		if got != "" {
			t.Errorf("FilterFromRange(%s) = %q, want empty for non-filterable type", name, got)
		}
	}
}

func TestFilterableTypesHaveDefaultTimePath(t *testing.T) {
	for _, dt := range Types() {
		if dt.Filterable && dt.DefaultTimePath == "" {
			t.Errorf("type %q has Filterable=true but empty DefaultTimePath", dt.EndpointName)
		}
	}
}

func TestKnownFilterableTypes(t *testing.T) {
	expected := []string{"weight", "heart-rate", "steps", "distance", "body-fat", "active-zone-minutes", "sedentary-period"}
	for _, name := range expected {
		dt, ok := Lookup(name)
		if !ok {
			t.Fatalf("Lookup(%q) failed", name)
		}
		if !dt.Filterable {
			t.Errorf("%s should be Filterable", name)
		}
		if dt.DefaultTimePath == "" {
			t.Errorf("%s has Filterable=true but empty DefaultTimePath", name)
		}
	}
}

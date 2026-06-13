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

func TestAZMHasListOperation(t *testing.T) {
	dt, ok := Lookup("active-zone-minutes")
	if !ok {
		t.Fatal("Lookup('active-zone-minutes') failed")
	}
	if !HasOperation(dt, "list") {
		t.Error("active-zone-minutes should have 'list' operation")
	}
	if !HasOperation(dt, "dailyRollUp") {
		t.Error("active-zone-minutes should have 'dailyRollUp' operation")
	}
}

func TestCapabilitiesIncludeAZMWithList(t *testing.T) {
	caps := Capabilities()
	for _, cap := range caps {
		if cap.Type == "active-zone-minutes" {
			if !cap.List {
				t.Error("active-zone-minutes capability should have list=true")
			}
			if !cap.Rollup {
				t.Error("active-zone-minutes capability should have rollup=true")
			}
			return
		}
	}
	t.Fatal("active-zone-minutes not found in capabilities")
}

func TestRollupCapableTypes(t *testing.T) {
	rollupTypes := []string{"steps", "distance", "active-zone-minutes", "total-calories"}
	for _, name := range rollupTypes {
		dt, ok := Lookup(name)
		if !ok {
			t.Fatalf("Lookup(%q) failed", name)
		}
		if !HasOperation(dt, "dailyRollUp") {
			t.Errorf("%s should have dailyRollUp operation", name)
		}
	}
}

func TestDailyTypesHaveClientTimePath(t *testing.T) {
	dailyTypes := []string{
		"daily-resting-heart-rate",
		"daily-heart-rate-variability",
		"daily-oxygen-saturation",
		"daily-respiratory-rate",
	}
	for _, name := range dailyTypes {
		dt, ok := Lookup(name)
		if !ok {
			t.Fatalf("Lookup(%q) failed", name)
		}
		if dt.ClientTimePath == "" {
			t.Errorf("%s should have ClientTimePath for client-side date filtering", name)
		}
	}
}

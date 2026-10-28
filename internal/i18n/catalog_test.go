package i18n

import (
	"regexp"
	"testing"
)

var printfVerb = regexp.MustCompile(`%[-+#0-9.]*[a-zA-Z]`)

func TestCatalogsComplete(t *testing.T) {
	required := make(map[string]bool, len(requiredCatalogKeys))
	for _, key := range requiredCatalogKeys {
		required[key] = true
	}
	for _, info := range Supported {
		if info.Code == EN {
			continue
		}
		catalog := catalogs[info.Code]
		if catalog == nil {
			t.Errorf("%s: catalog is not registered", info.Code)
			continue
		}
		for _, key := range requiredCatalogKeys {
			value := catalog[key]
			if value == "" {
				t.Errorf("%s: missing translation for %q", info.Code, key)
				continue
			}
			want := printfVerb.FindAllString(key, -1)
			got := printfVerb.FindAllString(value, -1)
			if len(want) != len(got) {
				t.Errorf("%s: %q has %d format verbs, translation has %d: %q", info.Code, key, len(want), len(got), value)
			}
		}
		for key := range catalog {
			if !required[key] {
				t.Errorf("%s: unexpected catalog key %q", info.Code, key)
			}
		}
	}
}

func TestPortugueseFallback(t *testing.T) {
	// pt-PT inherits anything it does not translate from pt-BR.
	key := "A key that only exists in the test"
	if T(PTPT, key) != key {
		t.Fatalf("missing key should fall back to English")
	}
	catalogs[PTPT][key] = ""
	catalogs[PTBR][key] = "chave"
	t.Cleanup(func() {
		delete(catalogs[PTPT], key)
		delete(catalogs[PTBR], key)
	})
	if got := T(PTPT, key); got != "chave" {
		t.Errorf("T(pt-PT) = %q, want pt-BR fallback", got)
	}
}

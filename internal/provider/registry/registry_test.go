package registry_test

import (
	"testing"

	_ "github.com/sunqirui1987/xhub/internal/provider/all"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
)

func TestBypassTypesStayWithTheirProvider(t *testing.T) {
	for _, typ := range providerregistry.Transports() {
		switch typ.ID {
		case "ark_contents_generation":
			if len(typ.Providers) != 1 || typ.Providers[0] != "volcengine" {
				t.Fatalf("ark providers %v", typ.Providers)
			}

		}
	}
}

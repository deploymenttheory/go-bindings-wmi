//go:build windows

package acceptance

import (
	"testing"

	"github.com/deploymenttheory/go-bindings-wmi/bindings/cim/securitycenter2"
	"github.com/deploymenttheory/go-bindings-wmi/bindings/cim/standardcimv2"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

// connectOrSkip connects to a namespace, skipping the test on hosts where it
// does not exist (root\SecurityCenter2 is client-SKU only; CI runners are
// Server). root\cimv2 tests elsewhere prove general connectivity.
func connectOrSkip(t *testing.T, namespace string) *wmi.Service {
	t.Helper()
	svc, err := wmi.Connect(namespace)
	if err != nil {
		t.Skipf("namespace %s unavailable on this host: %v", namespace, err)
	}
	return svc
}

func TestStandardCimv2NetAdapters(t *testing.T) {
	svc := connectOrSkip(t, `root\StandardCimv2`)
	defer svc.Close()

	adapters, err := standardcimv2.QueryMSFTNetAdapter(svc, "")
	if err != nil {
		t.Fatalf("QueryMSFTNetAdapter: %v", err)
	}
	if len(adapters) == 0 {
		t.Fatal("no network adapters")
	}
	if adapters[0].Name == "" {
		t.Errorf("adapter with empty Name: %+v", adapters[0])
	}
	t.Logf("%d adapters; first: %s (up=%v)", len(adapters), adapters[0].Name, adapters[0].State)

	addrs, err := standardcimv2.QueryMSFTNetIPAddress(svc, "")
	if err != nil {
		t.Fatalf("QueryMSFTNetIPAddress: %v", err)
	}
	if len(addrs) == 0 {
		t.Error("no IP addresses")
	}
}

func TestSecurityCenter2Products(t *testing.T) {
	svc := connectOrSkip(t, `root\SecurityCenter2`)
	defer svc.Close()

	products, err := securitycenter2.QueryAntiVirusProduct(svc, "")
	if err != nil {
		t.Fatalf("QueryAntiVirusProduct: %v", err)
	}
	// Every client SKU ships Defender at minimum.
	if len(products) == 0 {
		t.Error("no antivirus products registered")
	}
	for _, p := range products {
		t.Logf("AV: %s (state %#x)", p.DisplayName, p.ProductState)
		if p.DisplayName == "" {
			t.Error("antivirus product with empty DisplayName")
		}
	}
}

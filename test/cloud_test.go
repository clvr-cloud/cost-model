package test

import (
	"strings"
	"testing"

	"github.com/kubecost/cost-model/pkg/cloud"
	v1 "k8s.io/api/core/v1"
)

const (
	providerIDMap = "spec.providerID"
	nameMap       = "metadata.name"
	labelMapFoo   = "metadata.labels.foo"
)

func TestRegionValueFromMapField(t *testing.T) {
	wantRegion := "useast"
	wantpid := strings.ToLower("/subscriptions/0bd50fdf-c923-4e1e-850c-196dd3dcc5d3/resourceGroups/MC_test_test_eastus/providers/Microsoft.Compute/virtualMachines/aks-agentpool-20139558-0")
	providerIDWant := wantRegion + "," + wantpid

	n := &v1.Node{}
	n.Spec.ProviderID = "azure:///subscriptions/0bd50fdf-c923-4e1e-850c-196dd3dcc5d3/resourceGroups/MC_test_test_eastus/providers/Microsoft.Compute/virtualMachines/aks-agentpool-20139558-0"
	n.Labels = make(map[string]string)
	n.Labels[v1.LabelZoneRegion] = wantRegion
	got := cloud.NodeValueFromMapField(providerIDMap, n, true)
	if got != providerIDWant {
		t.Errorf("Assert on '%s' want '%s' got '%s'", providerIDMap, providerIDWant, got)
	}

}
func TestTransformedValueFromMapField(t *testing.T) {
	providerIDWant := "i-05445591e0d182d42"
	n := &v1.Node{}
	n.Spec.ProviderID = "aws:///us-east-1a/i-05445591e0d182d42"
	got := cloud.NodeValueFromMapField(providerIDMap, n, false)
	if got != providerIDWant {
		t.Errorf("Assert on '%s' want '%s' got '%s'", providerIDMap, providerIDWant, got)
	}

	providerIDWant2 := strings.ToLower("/subscriptions/0bd50fdf-c923-4e1e-850c-196dd3dcc5d3/resourceGroups/MC_test_test_eastus/providers/Microsoft.Compute/virtualMachines/aks-agentpool-20139558-0")
	n2 := &v1.Node{}
	n2.Spec.ProviderID = "azure:///subscriptions/0bd50fdf-c923-4e1e-850c-196dd3dcc5d3/resourceGroups/MC_test_test_eastus/providers/Microsoft.Compute/virtualMachines/aks-agentpool-20139558-0"
	got2 := cloud.NodeValueFromMapField(providerIDMap, n2, false)
	if got2 != providerIDWant2 {
		t.Errorf("Assert on '%s' want '%s' got '%s'", providerIDMap, providerIDWant2, got2)
	}

	providerIDWant3 := strings.ToLower("/subscriptions/0bd50fdf-c923-4e1e-850c-196dd3dcc5d3/resourceGroups/mc_testspot_testspot_eastus/providers/Microsoft.Compute/virtualMachineScaleSets/aks-nodepool1-19213364-vmss/virtualMachines/0")
	n3 := &v1.Node{}
	n3.Spec.ProviderID = "azure:///subscriptions/0bd50fdf-c923-4e1e-850c-196dd3dcc5d3/resourceGroups/mc_testspot_testspot_eastus/providers/Microsoft.Compute/virtualMachineScaleSets/aks-nodepool1-19213364-vmss/virtualMachines/0"
	got3 := cloud.NodeValueFromMapField(providerIDMap, n3, false)
	if got3 != providerIDWant3 {
		t.Errorf("Assert on '%s' want '%s' got '%s'", providerIDMap, providerIDWant3, got3)
	}
}

func TestNodeValueFromMapField(t *testing.T) {
	providerIDWant := "providerid"
	nameWant := "gke-standard-cluster-1-pool-1-91dc432d-cg69"
	labelFooWant := "labelfoo"

	n := &v1.Node{}
	n.Spec.ProviderID = providerIDWant
	n.Name = nameWant
	n.Labels = make(map[string]string)
	n.Labels["foo"] = labelFooWant

	got := cloud.NodeValueFromMapField(providerIDMap, n, false)
	if got != providerIDWant {
		t.Errorf("Assert on '%s' want '%s' got '%s'", providerIDMap, providerIDWant, got)
	}

	got = cloud.NodeValueFromMapField(nameMap, n, false)
	if got != nameWant {
		t.Errorf("Assert on '%s' want '%s' got '%s'", nameMap, nameWant, got)
	}

	got = cloud.NodeValueFromMapField(labelMapFoo, n, false)
	if got != labelFooWant {
		t.Errorf("Assert on '%s' want '%s' got '%s'", labelMapFoo, labelFooWant, got)
	}

}

func TestNodePriceFromCSV(t *testing.T) {
	providerIDWant := "providerid"
	nameWant := "gke-standard-cluster-1-pool-1-91dc432d-cg69"
	labelFooWant := "labelfoo"

	n := &v1.Node{}
	n.Spec.ProviderID = providerIDWant
	n.Name = nameWant
	n.Labels = make(map[string]string)
	n.Labels["foo"] = labelFooWant

	wantPrice := "0.1337"

	c := &cloud.CSVProvider{
		CSVLocation: "../configs/pricing_schema.csv",
		CustomProvider: &cloud.CustomProvider{
			Config: cloud.NewProviderConfig("../configs/default.json"),
		},
	}
	c.DownloadPricingData()
	k := c.GetKey(n.Labels, n)
	resN, err := c.NodePricing(k)
	if err != nil {
		t.Errorf("Error in NodePricing: %s", err.Error())
	} else {
		gotPrice := resN.Cost
		if gotPrice != wantPrice {
			t.Errorf("Wanted price '%s' got price '%s'", wantPrice, gotPrice)
		}
	}

	unknownN := &v1.Node{}
	unknownN.Spec.ProviderID = providerIDWant
	unknownN.Name = "unknownname"
	unknownN.Labels = make(map[string]string)
	unknownN.Labels["foo"] = labelFooWant
	k2 := c.GetKey(n.Labels, unknownN)
	resN2, _ := c.NodePricing(k2)
	if resN2 != nil {
		t.Errorf("CSV provider should return nil on missing node")
	}

	c2 := &cloud.CSVProvider{
		CSVLocation: "../configs/fake.csv",
		CustomProvider: &cloud.CustomProvider{
			Config: cloud.NewProviderConfig("../configs/default.json"),
		},
	}
	k3 := c.GetKey(n.Labels, n)
	resN3, _ := c2.NodePricing(k3)
	if resN3 != nil {
		t.Errorf("CSV provider should return nil on missing csv")
	}
}

func TestNodePriceFromCSVWithRegion(t *testing.T) {
	providerIDWant := "gke-standard-cluster-1-pool-1-91dc432d-cg69"
	nameWant := "foo"
	labelFooWant := "labelfoo"

	n := &v1.Node{}
	n.Spec.ProviderID = providerIDWant
	n.Name = nameWant
	n.Labels = make(map[string]string)
	n.Labels["foo"] = labelFooWant
	n.Labels[v1.LabelZoneRegion] = "regionone"
	wantPrice := "0.1337"

	n2 := &v1.Node{}
	n2.Spec.ProviderID = providerIDWant
	n2.Name = nameWant
	n2.Labels = make(map[string]string)
	n2.Labels["foo"] = labelFooWant
	n2.Labels[v1.LabelZoneRegion] = "regiontwo"
	wantPrice2 := "0.1338"

	n3 := &v1.Node{}
	n3.Spec.ProviderID = providerIDWant
	n3.Name = nameWant
	n3.Labels = make(map[string]string)
	n3.Labels["foo"] = labelFooWant
	n3.Labels[v1.LabelZoneRegion] = "fakeregion"
	wantPrice3 := "0.1339"

	c := &cloud.CSVProvider{
		CSVLocation: "../configs/pricing_schema_region.csv",
		CustomProvider: &cloud.CustomProvider{
			Config: cloud.NewProviderConfig("../configs/default.json"),
		},
	}
	c.DownloadPricingData()
	k := c.GetKey(n.Labels, n)
	resN, err := c.NodePricing(k)
	if err != nil {
		t.Errorf("Error in NodePricing: %s", err.Error())
	} else {
		gotPrice := resN.Cost
		if gotPrice != wantPrice {
			t.Errorf("Wanted price '%s' got price '%s'", wantPrice, gotPrice)
		}
	}
	k2 := c.GetKey(n2.Labels, n2)
	resN2, err := c.NodePricing(k2)
	if err != nil {
		t.Errorf("Error in NodePricing: %s", err.Error())
	} else {
		gotPrice := resN2.Cost
		if gotPrice != wantPrice2 {
			t.Errorf("Wanted price '%s' got price '%s'", wantPrice2, gotPrice)
		}
	}
	k3 := c.GetKey(n3.Labels, n3)
	resN3, err := c.NodePricing(k3)
	if err != nil {
		t.Errorf("Error in NodePricing: %s", err.Error())
	} else {
		gotPrice := resN3.Cost
		if gotPrice != wantPrice3 {
			t.Errorf("Wanted price '%s' got price '%s'", wantPrice3, gotPrice)
		}
	}

	unknownN := &v1.Node{}
	unknownN.Spec.ProviderID = "fake providerID"
	unknownN.Name = "unknownname"
	unknownN.Labels = make(map[string]string)
	unknownN.Labels["foo"] = labelFooWant
	k4 := c.GetKey(unknownN.Labels, unknownN)
	resN4, _ := c.NodePricing(k4)
	if resN4 != nil {
		t.Errorf("CSV provider should return nil on missing node, instead returned %+v", resN4)
	}

	c2 := &cloud.CSVProvider{
		CSVLocation: "../configs/fake.csv",
		CustomProvider: &cloud.CustomProvider{
			Config: cloud.NewProviderConfig("../configs/default.json"),
		},
	}
	k5 := c.GetKey(n.Labels, n)
	resN5, _ := c2.NodePricing(k5)
	if resN5 != nil {
		t.Errorf("CSV provider should return nil on missing csv")
	}
}

func TestNodePriceFromCSVWithCase(t *testing.T) {
	n := &v1.Node{}
	n.Spec.ProviderID = "azure:///subscriptions/123a7sd-asd-1234-578a9-123abcdef/resourceGroups/case_12_STaGe_TeSt7/providers/Microsoft.Compute/virtualMachineScaleSets/vmss-agent-worker0-12stagetest7-ezggnore/virtualMachines/7"
	n.Labels = make(map[string]string)
	n.Labels[v1.LabelZoneRegion] = "eastus2"
	wantPrice := "0.13370357"

	c := &cloud.CSVProvider{
		CSVLocation: "../configs/pricing_schema_case.csv",
		CustomProvider: &cloud.CustomProvider{
			Config: cloud.NewProviderConfig("../configs/default.json"),
		},
	}

	c.DownloadPricingData()
	k := c.GetKey(n.Labels, n)
	resN, err := c.NodePricing(k)
	if err != nil {
		t.Errorf("Error in NodePricing: %s", err.Error())
	} else {
		gotPrice := resN.Cost
		if gotPrice != wantPrice {
			t.Errorf("Wanted price '%s' got price '%s'", wantPrice, gotPrice)
		}
	}

}

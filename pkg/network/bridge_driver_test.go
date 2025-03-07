package network

import (
	"net"
	"testing"
)

var testName = "testbridge"

func TestBridgeNetworkDriver_Create(t *testing.T) {
	b := BridgeNetworkDriver{}
	n, err := b.Create("192.168.0.1/24", testName)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("create network: %v", n)
}

func TestBridgeNetworkDriver_Delete(t *testing.T) {
	b := BridgeNetworkDriver{}
	_, ipRange, _ := net.ParseCIDR("192.168.0.1/24")
	n := &Network{
		Name:    testName,
		IPRange: ipRange,
	}
	err := b.Delete(n)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("delete network: %v", testName)
}

func TestBridgeNetworkDriver_Connect(t *testing.T) {
	ep := &Endpoint{
		ID: "testcontaier",
	}
	n := &Network{
		Name: testName,
	}
	b := BridgeNetworkDriver{}
	err := b.Connect(n, ep)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBridgeNetworkDriver_Disconnect(t *testing.T) {
	ep := &Endpoint{
		ID: "testcontainer",
	}
	//n := &Network{
	//	Name: testName,
	//}
	b := BridgeNetworkDriver{}
	err := b.Disconnect(ep)
	if err != nil {
		t.Fatal(err)
	}
}

package main

import "testing"

func TestDefaultAddressUsesLoopback(t *testing.T) {
	address, err := defaultAddress("")
	if err != nil {
		t.Fatal(err)
	}
	if address != "127.0.0.1:19081" {
		t.Fatalf("默认地址 %s", address)
	}
	address, err = defaultAddress("19123")
	if err != nil {
		t.Fatal(err)
	}
	if address != "127.0.0.1:19123" {
		t.Fatalf("PORT 地址 %s", address)
	}
	if _, err := defaultAddress("invalid"); err == nil {
		t.Fatal("无效 PORT 未被拒绝")
	}
}

package discovery

import (
	"fmt"
	"testing"
)

func TestGetAddr(t *testing.T) {
	localIP, err := GetLocalIP()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(localIP)
}

func TestLocalIP(t *testing.T) {
	ip, err := GetLocalIP()
	fmt.Printf("%s %v", ip, err)
}

func TestPublicIP(t *testing.T) {
	ip, err := GetPublicIP()
	fmt.Printf("%s %v", ip, err)
}

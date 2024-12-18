package discovery

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no ip address found")
}

func GetPublicIP() (string, error) {
	var wg sync.WaitGroup
	urls := []string{
		"https://checkip.amazonaws.com",
		// "https://ident.me",
		// "https://ifconfig.cc/ip",
		// "https://ipinfo.io/ip",
		// "https://ifconfig.co/ip",
		// "https://ifconfig.io/ip",
		// "https://ifconfig.me/ip",
	}
	ch := make(chan string, len(urls))

	reqIp := func(apiURL string) (string, error) {
		resp, err := http.Get(apiURL)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("status code: %d", resp.StatusCode)
		}
		ip, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return strings.Trim(string(ip), "\n"), nil
	}

	for _, url := range urls {
		wg.Add(1)
		go func(apiURL string) {
			defer wg.Done()
			ip, err := reqIp(apiURL)
			if err == nil {
				ch <- ip
			}
		}(url)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for ip := range ch {
		return ip, nil
	}
	return "", fmt.Errorf("no ip address found")
}

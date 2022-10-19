package main

import (
	"errors"
	"net"
	"os"
	"strings"

	"github.com/gomodule/redigo/redis"
)

// 取代 keys 效能更好
func RedisScan(c redis.Conn, match string) (keys []string, err error) {
	iter := 0

	for {

		if arr, err := redis.Values(c.Do("SCAN", iter, "MATCH", match, "COUNT", 100)); err != nil {
			return nil, err
		} else {

			iter, _ = redis.Int(arr[0], nil)
			keys, _ = redis.Strings(arr[1], nil)
		}

		if iter == 0 {
			break
		}
	}
	return
}

func GetMacAddr() (addr string, err error) {
	ifas, err := net.Interfaces()
	if err != nil {
		return
	}
	var as []string
	for _, ifa := range ifas {
		a := ifa.HardwareAddr.String()
		if a != "" {
			as = append(as, a)
		}
	}
	addr = strings.Join(as, ",")
	return
}

func GetHostname() (string, error) {
	return os.Hostname()
}

func GetExternalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}

package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"unicode"
)

type Config struct {
	Port       int
	IP         string
	MaxUpload  int64
	MaxSizeStr string
	ShowLogs   bool
	SharedRoot string
	Secure     bool
	ReadOnly   bool
}

func ParseSize(sizeStr string) (int64, error) {
	sizeStr = strings.TrimSpace(sizeStr)
	if sizeStr == "" {
		return 0, errors.New("empty size string")
	}

	var numStr string
	var unit string
	for i, r := range sizeStr {
		if !unicode.IsDigit(r) && r != '.' {
			numStr = sizeStr[:i]
			unit = strings.TrimSpace(sizeStr[i:])
			break
		}
	}
	if numStr == "" {
		numStr = sizeStr
	}

	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number format in size %q: %w", sizeStr, err)
	}

	unit = strings.ToUpper(unit)
	var multiplier int64 = 1

	switch unit {
	case "B", "":
		multiplier = 1
	case "K", "KB":
		multiplier = 1024
	case "M", "MB":
		multiplier = 1024 * 1024
	case "G", "GB":
		multiplier = 1024 * 1024 * 1024
	case "T", "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown size unit %q", unit)
	}

	return int64(val * float64(multiplier)), nil
}

func AutoDetectIP() string {
	conn, err := net.Dial("udp", "1.1.1.1:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		ip := localAddr.IP.String()
		if ip != "" && ip != "0.0.0.0" {
			return ip
		}
	}

	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String()
				}
			}
		}
	}

	return "127.0.0.1"
}

func CheckPortAvailable(ip string, port int) bool {
	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func GetAvailablePort(ip string, startPort int, maxTries int) (int, error) {
	for i := 0; i < maxTries; i++ {
		port := startPort + i
		if CheckPortAvailable(ip, port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available TCP port found starting from %d after %d tries", startPort, maxTries)
}

func GetSharedRoot() (string, error) {
	root := os.Getenv("SHARE_ROOT")
	if root != "" {
		return root, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	return cwd, nil
}

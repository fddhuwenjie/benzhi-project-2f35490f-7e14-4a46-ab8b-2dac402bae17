package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	Addr      string
	Database  string
	SelfCheck bool
}

func parseConfig(args []string) (config, error) {
	defaults, err := defaultAddress(os.Getenv("PORT"))
	if err != nil {
		return config{}, err
	}
	set := flag.NewFlagSet("sheltergate", flag.ContinueOnError)
	var cfg config
	set.StringVar(&cfg.Addr, "addr", defaults, "HTTP 监听地址")
	set.StringVar(&cfg.Database, "db", "sheltergate.db", "SQLite 数据库文件")
	set.BoolVar(&cfg.SelfCheck, "selfcheck", false, "执行完整 HTTP 自检后退出")
	if err := set.Parse(args); err != nil {
		return config{}, err
	}
	if set.NArg() != 0 {
		return config{}, fmt.Errorf("存在未识别参数: %s", strings.Join(set.Args(), " "))
	}
	if err := validateAddress(cfg.Addr); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(cfg.Database) == "" {
		return config{}, fmt.Errorf("数据库路径不能为空")
	}
	return cfg, nil
}

func defaultAddress(portValue string) (string, error) {
	if portValue == "" {
		return "127.0.0.1:19081", nil
	}
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), nil
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return fmt.Errorf("-addr 必须为 host:port 格式")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("-addr 端口无效")
	}
	return nil
}

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 5 * time.Second
)

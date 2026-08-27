package main

import (
	"fmt"
	"os"
)

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err == nil {
		if cfg.SelfCheck {
			err = runSelfCheck(cfg)
		} else {
			err = runServer(cfg)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "sheltergate:", err)
		os.Exit(1)
	}
}

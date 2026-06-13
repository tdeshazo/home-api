package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/tdeshazo/home-api/internal/auth"
)

func main() {
	password := flag.String("password", "", "password to hash; if empty, password is read from stdin")
	flag.Parse()

	if flag.NArg() != 0 {
		log.Fatal("unexpected positional arguments")
	}

	raw := *password
	if raw == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("read password from stdin: %v", err)
		}
		raw = strings.TrimRight(string(data), "\r\n")
	}
	if raw == "" {
		log.Fatal("password is required via -password or stdin")
	}

	hash, err := auth.HashPassword(raw)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	fmt.Println(hash)
}

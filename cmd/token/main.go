package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"social-api/internal/auth"

	"github.com/google/uuid"
)

func main() {
	userID := flag.String("user-id", "", "user UUID to place in the JWT subject")
	secret := flag.String("secret", "", "HS256 signing secret")
	issuer := flag.String("issuer", "social-api", "JWT issuer")
	audience := flag.String("audience", "social-api-api", "JWT audience")
	ttl := flag.Duration("ttl", time.Hour, "token lifetime")
	flag.Parse()

	if *userID == "" {
		log.Fatal("-user-id is required")
	}
	if *secret == "" {
		log.Fatal("-secret is required")
	}

	id, err := uuid.Parse(*userID)
	if err != nil {
		log.Fatalf("parse user id: %v", err)
	}

	signed, err := auth.MakeJWTWithClaims(id, *secret, *ttl, *issuer, *audience)
	if err != nil {
		log.Fatalf("sign token: %v", err)
	}

	fmt.Println(signed)
}

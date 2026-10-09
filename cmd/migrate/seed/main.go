package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/farrasnazhif/ripple-be/internal/database"
	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	addr := os.Getenv("DB_ADDR")
	if addr == "" {
		addr = "postgres://admin:adminpassword@localhost/ripple-be?sslmode=disable"
	}

	db, err := database.New(database.Config{Addr: addr, MaxOpenConns: 3, MaxIdleConns: 3, MaxIdleTime: "15m"})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
		user := &entity.User{
			Username:     fmt.Sprintf("user%d", i),
			Email:        fmt.Sprintf("user%d@example.com", i),
			PasswordHash: hash,
			IsActive:     true,
		}
		if err := userRepo.Create(ctx, user); err != nil {
			log.Printf("skip user%d: %v", i, err)
			continue
		}
	}
	log.Println("seeding complete")
}

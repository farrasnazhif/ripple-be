package service

import (
	"context"
	"log"
	"time"

	"github.com/farrasnazhif/ripple-be/internal/repository"
)

func CleanupStorage(repo *repository.StorageCleanupRepository, storage BriefStorage) {
	if storage == nil {
		return
	}
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		keys, err := repo.Pending(ctx)
		if err != nil {
			log.Println("could not read pending storage cleanup")
		}
		for _, key := range keys {
			if err := storage.Delete(ctx, key); err != nil {
				log.Println("storage cleanup failed; will retry")
				continue
			}
			if err := repo.Complete(ctx, key); err != nil {
				log.Println("could not acknowledge storage cleanup; will retry")
			}
		}
		cancel()
		<-tick.C
	}
}

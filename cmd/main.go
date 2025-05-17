package main

import (
	"log"
	"net"

	"github.com/yago-123/cidrx"
)

const AllocateCount = 5

func main() {
	pool, err := cidrx.NewPool("2001:db8::", 64, 120, 1024)
	if err != nil {
		log.Printf("Error creating pool: %s", err)
		return
	}

	ips := make([]net.IP, AllocateCount)
	for i := 0; i < AllocateCount; i++ {
		ip, errAllocation := pool.Allocate()
		if errAllocation != nil {
			log.Printf("Error allocating IP: %s", errAllocation)
			continue
		}
		log.Printf("Allocated IP: %s", ip.String())

		ips[i] = ip
	}

	log.Printf("Taking snapshot of the pool")
	snapshot := pool.Snapshot()

	for _, ip := range ips {
		if errRelease := pool.Release(ip); errRelease != nil {
			log.Printf("Error releasing IP: %s", errRelease)
		}

		log.Printf("Released IP: %s", ip)
	}

	log.Printf("Recreating pool from snapshot")
	newPool, err := cidrx.NewPoolFromSnapshot(snapshot)
	if err != nil {
		log.Printf("Error creating pool from snapshot: %s", err)
		return
	}

	for _, ip := range ips {
		if errRelease := newPool.Release(ip); errRelease != nil {
			log.Printf("Error releasing IP from snapshot: %s", errRelease)
		}

		log.Printf("Released IP from snapshot: %s", ip)
	}
}

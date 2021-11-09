package cache

import (
	"log"
	"testing"
	"time"
)

func TestCache(t *testing.T) {
	key := "test"
	value := "test_value"
	c := NewTTLCache(5 * time.Second)
	c.Set(key, value)

	time.Sleep(2 * time.Second)

	item, exists := c.Get(key)
	log.Printf("2s item: %v, exists: %v", item, exists)

	for i := 0; i <= 5; i++ {
		time.Sleep(7 * time.Second)
		item, exists := c.Get("key1")
		log.Printf("time: %v item: %v, exists: %v", time.Now(), item, exists)
	}

}

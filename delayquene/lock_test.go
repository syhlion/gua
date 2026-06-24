package delayquene

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestLockExclusivityAndCAS(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	pool := testPool(mr.Addr(), 0)
	c := pool.Get()
	defer c.Close()

	ok, err := acquireLock(c, "K", "t1", 10000)
	if err != nil || !ok {
		t.Fatalf("first acquire should succeed: ok=%v err=%v", ok, err)
	}
	// held by t1 -> t2 cannot acquire
	if ok, _ := acquireLock(c, "K", "t2", 10000); ok {
		t.Fatal("second acquire should fail while held")
	}
	// release with wrong token is a no-op (CAS)
	releaseLock(c, "K", "t2")
	if ok, _ := acquireLock(c, "K", "t3", 10000); ok {
		t.Fatal("wrong-token release must not free the lock")
	}
	// release with the right token frees it
	releaseLock(c, "K", "t1")
	if ok, _ := acquireLock(c, "K", "t3", 10000); !ok {
		t.Fatal("correct-token release should free the lock")
	}
}

func TestLockTTLExpiry(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	pool := testPool(mr.Addr(), 0)
	c := pool.Get()
	defer c.Close()

	if ok, _ := acquireLock(c, "K", "owner", 50); !ok {
		t.Fatal("acquire should succeed")
	}
	// holder "dies" without releasing; TTL must free the lock.
	mr.FastForward(100 * time.Millisecond)
	if ok, _ := acquireLock(c, "K", "other", 50); !ok {
		t.Fatal("lock should auto-release after TTL (no deadlock on holder crash)")
	}
}

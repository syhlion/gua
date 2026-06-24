package delayquene

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gomodule/redigo/redis"
)

// newToken returns a random owner token for a distributed lock.
func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// releaseScript deletes the lock key only if it still holds our token, so a
// lock that already expired and was re-acquired by someone else is never
// stolen-deleted.
var releaseScript = redis.NewScript(1, `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0`)

// heartbeatScript refreshes a server slot's heartbeat only while this node
// still owns it (fencing): if the owner token no longer matches, the slot was
// reclaimed by another node during a long pause and we must NOT keep using its
// snowflake id. KEYS[1]=SERVER-N-OWN KEYS[2]=SERVER-N ARGV[1]=token ARGV[2]=now.
var heartbeatScript = redis.NewScript(2, `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	redis.call("SET", KEYS[2], ARGV[2])
	return 1
end
return 0`)

// acquireLock attempts SET key token NX PX ttlMs. Returns true if acquired.
// Unlike a bare SETNX, the PX expiry guarantees the lock self-releases if the
// holder dies, so a crash can never deadlock the cluster.
func acquireLock(c redis.Conn, key, token string, ttlMs int) (bool, error) {
	reply, err := redis.String(c.Do("SET", key, token, "NX", "PX", ttlMs))
	if err == redis.ErrNil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return reply == "OK", nil
}

// releaseLock removes the lock iff we still own it (token match).
func releaseLock(c redis.Conn, key, token string) {
	_, _ = releaseScript.Do(c, key, token)
}

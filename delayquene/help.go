package delayquene

import (
	"github.com/gomodule/redigo/redis"
)

// 取代 keys 效能更好
func RedisScan(c redis.Conn, match string) (keys []string, err error) {
	iter := 0

	for {

		if arr, err := redis.Values(c.Do("SCAN", iter, "MATCH", match, "COUNT", 100)); err != nil {
			return nil, err
		} else {

			iter, _ = redis.Int(arr[0], nil)
			keys, _ = redis.Strings(arr[1], nil)
		}

		if iter == 0 {
			break
		}
	}
	return
}

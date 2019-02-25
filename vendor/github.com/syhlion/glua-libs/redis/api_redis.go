package redis

import (
	"sync"

	"github.com/gomodule/redigo/redis"
)

type luaRedis struct {
	sync.Mutex
	pool *redis.Pool
}

func (l *luaRedis) constructor(conn string, db int) (lRedis, error) {
	pool := &redis.Pool{
		// Other pool configuration not shown in this example.
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", conn)
			if err != nil {
				return nil, err
			}
			if _, err := c.Do("SELECT", db); err != nil {
				c.Close()
				return nil, err
			}
			return c, nil
		},
	}
	result := &luaRedis{}
	pool.MaxIdle = MaxIdleConns
	pool.MaxActive = MaxOpenConns
	result.pool = pool
	c := pool.Get()
	defer c.Close()
	_, err := c.Do("PING")
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (l *luaRedis) getPool() *redis.Pool {
	l.Lock()
	defer l.Unlock()
	return l.pool
}

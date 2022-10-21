package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"regexp"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	guaproto "github.com/syhlion/gua/proto"
)

type Migrate struct {
	groupRedis *redis.Pool
	delayRedis *redis.Pool
	apiRedis   *redis.Pool
}

func (m *Migrate) Dump(groupName string) (buf *bytes.Buffer, err error) {

	apiBackup, err := m.apiBackup(groupName)
	if err != nil {
		return
	}
	groupBackup, err := m.groupBackup(groupName)
	if err != nil {
		return buf, err
	}
	delayBackup, err := m.delayBackup(groupName)
	if err != nil {
		return buf, err
	}
	buf = new(bytes.Buffer)
	gz := gzip.NewWriter(buf)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	for k, v := range apiBackup {
		hdr := &tar.Header{
			Name: k,
			Mode: 0600,
			Size: int64(len(v)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return buf, err
		}
		if _, err := tw.Write(v); err != nil {
			return buf, err
		}
	}
	for k, v := range groupBackup {
		hdr := &tar.Header{
			Name: k,
			Mode: 0600,
			Size: int64(len(v)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return buf, err
		}
		if _, err := tw.Write(v); err != nil {
			return buf, err
		}
	}
	for k, v := range delayBackup {
		hdr := &tar.Header{
			Name: k,
			Mode: 0600,
			Size: int64(len(v)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return buf, err
		}
		if _, err := tw.Write(v); err != nil {
			return buf, err
		}
	}

	return
}
func (m *Migrate) groupBackup(groupName string) (backup map[string][]byte, err error) {
	conn := m.groupRedis.Get()
	defer conn.Close()
	groupKeys, err := RedisScan(conn, "USER_"+groupName)
	if err != nil {
		return
	}
	backup = make(map[string][]byte)
	for _, v := range groupKeys {
		if err != nil {
			return nil, err
		}
		body, err := redis.Bytes(conn.Do("GET", v))
		if err != nil {
			return nil, err
		}
		backup[v] = body

	}
	nodeKeys, err := RedisScan(conn, "REMOTE_NODE_"+groupName)
	if err != nil {
		return nil, err
	}
	for _, v := range nodeKeys {
		if err != nil {
			return nil, err
		}
		body, err := redis.Bytes(conn.Do("GET", v))
		if err != nil {
			return nil, err
		}
		backup[v] = body

	}
	return
}
func (m *Migrate) delayBackup(groupName string) (backup map[string][]byte, err error) {
	conn := m.delayRedis.Get()
	defer conn.Close()
	jobKeys, err := RedisScan(conn, "JOB-"+groupName)
	if err != nil {
		return nil, err
	}
	backup = make(map[string][]byte)
	for _, v := range jobKeys {
		if err != nil {
			return nil, err
		}
		body, err := redis.Bytes(conn.Do("GET", v))
		if err != nil {
			return nil, err
		}
		backup[v] = body

	}
	return
}
func (m *Migrate) apiBackup(groupName string) (backup map[string][]byte, err error) {
	conn := m.apiRedis.Get()
	defer conn.Close()
	apiKeys, err := RedisScan(conn, "FUNC-"+groupName)
	if err != nil {
		return nil, err
	}
	backup = make(map[string][]byte)
	for _, v := range apiKeys {
		if err != nil {
			return nil, err
		}
		body, err := redis.Bytes(conn.Do("GET", v))
		if err != nil {
			return nil, err
		}
		backup[v] = body

	}
	return
}

var (
	jobRe   = regexp.MustCompile(`^JOB-(.+)-(\d+)$`)
	groupRe = regexp.MustCompile("^USER_")
	funcRe  = regexp.MustCompile("^FUNC-")
	nodeRe  = regexp.MustCompile("^REMOTE_NODE_")
)

func (m *Migrate) Import(b []byte) (err error) {
	bb := bytes.NewReader(b)
	gz, err := gzip.NewReader(bb)
	if err != nil {
		return
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		if groupRe.MatchString(h.Name) {
			err = func() (err error) {
				conn := m.groupRedis.Get()
				defer conn.Close()
				buf := new(bytes.Buffer)
				buf.ReadFrom(tr)
				_, err = conn.Do("SET", h.Name, string(buf.Bytes()))
				if err != nil {
					return
				}
				return
			}()
			if err != nil {
				return err
			}
			continue

		}
		if jobRe.MatchString(h.Name) {
			err = func() (err error) {
				conn := m.delayRedis.Get()
				defer conn.Close()
				buf := new(bytes.Buffer)
				buf.ReadFrom(tr)

				job := &guaproto.Job{}
				err = proto.Unmarshal(buf.Bytes(), job)
				if err != nil {
					return
				}
				job.Active = true

				b, err := proto.Marshal(job)
				if err != nil {
					return
				}

				_, err = conn.Do("SET", h.Name, b)
				if err != nil {
					return
				}
				_, err = conn.Do("SET", h.Name+"-scan", time.Now())
				if err != nil {
					return
				}
				return
			}()
			if err != nil {
				return err
			}

			continue
		}
		if funcRe.MatchString(h.Name) {
			err = func() (err error) {
				conn := m.apiRedis.Get()
				defer conn.Close()
				buf := new(bytes.Buffer)
				buf.ReadFrom(tr)

				ff := &guaproto.Func{}
				err = proto.Unmarshal(buf.Bytes(), ff)
				if err != nil {
					return
				}

				b, err := proto.Marshal(ff)
				if err != nil {
					return
				}

				_, err = conn.Do("SET", h.Name, b)
				if err != nil {
					return
				}
				return
			}()
			if err != nil {
				return err
			}
			continue
		}
		if nodeRe.MatchString(h.Name) {
			err = func() (err error) {
				conn := m.groupRedis.Get()
				defer conn.Close()
				buf := new(bytes.Buffer)
				buf.ReadFrom(tr)

				ff := &guaproto.NodeRegisterRequest{}
				err = proto.Unmarshal(buf.Bytes(), ff)
				if err != nil {
					return
				}

				b, err := proto.Marshal(ff)
				if err != nil {
					return
				}

				_, err = conn.Do("SET", h.Name, b)
				if err != nil {
					return
				}
				return
			}()
			if err != nil {
				return err
			}
			continue
		}

	}

	return
}

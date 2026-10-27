package nosql

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/transports"
	"github.com/redis/go-redis/v9"
)

type redisBackend struct {
	cfg       entities.ConnectionConfig
	transport transports.Transport
	client    *redis.Client
}

func (b *redisBackend) connect(ctx context.Context) error {
	db, err := strconv.Atoi(b.cfg.Database)
	if err != nil || db < 0 {
		return fmt.Errorf("Redis database must be a non-negative integer (usually 0)")
	}
	opts := &redis.Options{Addr: b.transport.Address(), Username: b.cfg.User, Password: b.cfg.Password, DB: db, DialTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, ContextTimeoutEnabled: true, MaxRetries: -1}
	if b.cfg.SSL {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: b.cfg.Host}
	}
	b.client = redis.NewClient(opts)
	return b.ping(ctx)
}
func (b *redisBackend) close() error {
	if b.client != nil {
		return b.client.Close()
	}
	return nil
}
func (b *redisBackend) ping(ctx context.Context) error {
	if b.client == nil {
		return fmt.Errorf("Redis is disconnected")
	}
	return b.client.Ping(ctx).Err()
}

// A finite-command allowlist excludes pooled-connection state (SELECT, AUTH,
// MULTI), subscriptions, blocking operations and server-side code execution.
const redisReads = " PING ECHO GET MGET STRLEN GETRANGE EXISTS TYPE TTL PTTL DUMP HGET HMGET HGETALL HLEN HKEYS HVALS HEXISTS HSTRLEN HRANDFIELD HSCAN LRANGE LLEN LINDEX SCARD SISMEMBER SMISMEMBER SMEMBERS SRANDMEMBER SSCAN ZCARD ZCOUNT ZLEXCOUNT ZRANGE ZREVRANGE ZRANGEBYSCORE ZREVRANGEBYSCORE ZRANGEBYLEX ZRANK ZREVRANK ZSCORE ZMSCORE ZSCAN XLEN XRANGE XREVRANGE DBSIZE INFO TIME SCAN "
const redisWrites = " SET MSET MSETNX SETNX SETEX PSETEX GETSET GETDEL GETEX APPEND INCR INCRBY INCRBYFLOAT DECR DECRBY DEL UNLINK EXPIRE PEXPIRE EXPIREAT PEXPIREAT PERSIST RENAME RENAMENX HSET HSETNX HMSET HDEL HINCRBY HINCRBYFLOAT LPUSH RPUSH LPUSHX RPUSHX LPOP RPOP LSET LTRIM LREM LMOVE SADD SREM SPOP SMOVE ZADD ZREM ZINCRBY ZREMRANGEBYRANK ZREMRANGEBYSCORE ZREMRANGEBYLEX ZPOPMIN ZPOPMAX XADD XDEL XTRIM "

func redisCommand(q string) ([]any, bool, error) {
	var args []any
	dec := json.NewDecoder(strings.NewReader(q))
	dec.UseNumber()
	if err := dec.Decode(&args); err != nil || len(args) == 0 {
		return nil, false, fmt.Errorf(`enter a Redis command as a JSON array, for example ["GET", "key"]`)
	}
	// Reject trailing JSON so an accidental second command is not silently ignored.
	if !json.Valid([]byte(q)) {
		return nil, false, fmt.Errorf("enter one JSON command array")
	}
	for i, arg := range args {
		switch v := arg.(type) {
		case string:
		case json.Number:
			args[i] = v.String()
		default:
			return nil, false, fmt.Errorf("Redis arguments must be strings or numbers")
		}
	}
	name, ok := args[0].(string)
	if !ok {
		return nil, false, fmt.Errorf("Redis command name must be a string")
	}
	name = strings.ToUpper(name)
	args[0] = name
	read := strings.Contains(redisReads, " "+name+" ")
	if !read && !strings.Contains(redisWrites, " "+name+" ") {
		return nil, false, fmt.Errorf("Redis command %q is not supported in the result grid", name)
	}
	return args, read, nil
}
func (b *redisBackend) query(ctx context.Context, q string, size int, state []byte) (*entities.QueryResult, []byte, error) {
	args, read, err := redisCommand(q)
	if err != nil {
		return nil, nil, err
	}
	if b.cfg.ReadOnly && !read {
		return nil, nil, fmt.Errorf("this connection is read-only")
	}
	if args[0] == "SCAN" {
		// SCAN is a native cursor, not an offset: COUNT is only a server hint.
		if len(args) < 2 {
			return nil, nil, fmt.Errorf(`use ["SCAN", "0", "MATCH", "*"]`)
		}
		scan, err := strconv.ParseUint(fmt.Sprint(args[1]), 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid SCAN cursor")
		}
		match := "*"
		kind := ""
		for i := 2; i < len(args); i += 2 {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("SCAN options require a value")
			}
			switch strings.ToUpper(fmt.Sprint(args[i])) {
			case "MATCH":
				match = fmt.Sprint(args[i+1])
			case "TYPE":
				kind = fmt.Sprint(args[i+1])
			case "COUNT":
				if n, e := strconv.Atoi(fmt.Sprint(args[i+1])); e != nil || n < 1 {
					return nil, nil, fmt.Errorf("invalid SCAN count")
				}
			default:
				return nil, nil, fmt.Errorf("unsupported SCAN option")
			}
		}
		if len(state) > 0 {
			scan, err = strconv.ParseUint(string(state), 10, 64)
			if err != nil {
				return nil, nil, err
			}
		}
		var keys []string
		var next uint64
		if kind != "" {
			keys, next, err = b.client.ScanType(ctx, scan, match, int64(size), kind).Result()
		} else {
			keys, next, err = b.client.Scan(ctx, scan, match, int64(size)).Result()
		}
		if err != nil {
			return nil, nil, err
		}
		result := &entities.QueryResult{IsQuery: true, Columns: []entities.ColumnInfo{{Name: "key", DatabaseType: "string"}}, Rows: [][]any{}}
		for _, key := range keys {
			result.Rows = append(result.Rows, []any{key})
		}
		var token []byte
		if next != 0 {
			token = []byte(strconv.FormatUint(next, 10))
		}
		return result, token, nil
	}
	if len(state) > 0 {
		return nil, nil, fmt.Errorf("this command does not support automatic paging")
	}
	value, err := b.client.Do(ctx, args...).Result()
	if errors.Is(err, redis.Nil) {
		value = nil
		err = nil
	}
	if err != nil {
		return nil, nil, err
	}
	value = redisValue(value)
	result := valueResult(value)
	if values, ok := value.([]any); ok {
		result.Rows = [][]any{}
		for _, v := range values {
			result.Rows = append(result.Rows, []any{cqlValue(v)})
		}
	}
	return result, nil, nil
}

// RESP3 maps have interface keys. Normalize them before JSON encoding so hashes
// and nested stream values remain inspectable instead of becoming Go map text.
func redisValue(value any) any {
	switch v := value.(type) {
	case map[any]any:
		result := make(map[string]any, len(v))
		for key, item := range v {
			result[fmt.Sprint(key)] = redisValue(item)
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, item := range v {
			result[key] = redisValue(item)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = redisValue(item)
		}
		return result
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return value
	}
}
func (b *redisBackend) tables(context.Context) ([]entities.InspectTableInfo, error) {
	return collection(b.cfg, "Keys"), nil
}
func (b *redisBackend) columns(context.Context, entities.InspectTableInfo) ([]entities.InspectColumnInfo, error) {
	return []entities.InspectColumnInfo{{Name: "key", DatabaseType: "string"}}, nil
}
func (b *redisBackend) ddl(context.Context, entities.TableRef) (string, error) {
	return "", fmt.Errorf("Redis has no DDL; use TYPE, TTL and native commands to inspect keys")
}

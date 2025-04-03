package task

import (
	"fmt"
	"github.com/go-redis/redis"
	"inspect/pkg/common"
	"strings"
)

type DBTask struct {
	Task

	rdsClient   RDSClient
	redisClient *redis.Client

	redisInfo map[string]string
}

func (t *DBTask) Init(opts *Options) error {
	t.Options = opts
	t.result = make(map[string]interface{})
	if opts.EnableRedis {
		t.redisClient = opts.GetRedisClient()
	}

	if opts.EnableRDS {
		client, err := opts.GetRDSClient()
		if err != nil {
			return err
		}
		t.rdsClient = client
	}
	return nil
}

func (t *DBTask) Get(key string) string {
	if v, exist := t.redisInfo[key]; exist {
		return strings.TrimSpace(v)
	} else {
		return common.Empty
	}
}

func (t *DBTask) GetTableInfo() error {
	if result, err := t.rdsClient.GetTableInfo(); err != nil {
		return err
	} else {
		t.result["Top10Table"] = result
	}
	return nil
}

func (t *DBTask) GetDBInfo() error {
	if info, err := t.rdsClient.GetRDSInfo(); err != nil {
		return err
	} else {
		t.result["DBInfo"] = info
	}
	return nil
}

func (t *DBTask) GetRDSInfo() error {
	t.result["HasRDSInfo"] = t.Options.EnableRDS
	if !t.Options.EnableRDS {
		return nil
	}
	if err := t.GetTableInfo(); err != nil {
		return err
	}
	if err := t.GetDBInfo(); err != nil {
		return err
	}
	return nil
}

func (t *DBTask) SetRedisInfoFromServer() error {
	infoStr, err := t.redisClient.Info().Result()
	if err != nil {
		return fmt.Errorf("获取 Redis 的 info 信息失败: %s", err)
	}
	info := make(map[string]string)
	lines := strings.Split(infoStr, "\n")
	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, "#") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				info[parts[0]] = parts[1]
			}
		}
	}
	t.redisInfo = info
	return nil
}

func (t *DBTask) GetRedisInfo() error {
	t.result["HasRedisInfo"] = t.Options.EnableRedis
	if !t.Options.EnableRedis {
		return nil
	}

	err := t.SetRedisInfoFromServer()
	if err != nil {
		return err
	}
	// service info
	t.result["RedisVersion"] = t.Get("redis_version")
	t.result["RedisMode"] = t.Get("redis_mode")
	t.result["RedisPort"] = t.Get("tcp_port")
	t.result["RedisUptime"] = t.Get("uptime_in_days")

	// client info
	t.result["RedisConnect"] = t.Get("connected_clients")
	t.result["RedisClusterConnect"] = t.Get("cluster_connections")
	t.result["RedisMaxConnect"] = t.Get("maxclients")
	t.result["RedisBlockedConnect"] = t.Get("blocked_clients")

	// memory info
	t.result["UsedMemoryHuman"] = t.Get("used_memory_human")
	t.result["UsedMemoryRssHuman"] = t.Get("used_memory_rss_human")
	t.result["UsedMemoryPeakHuman"] = t.Get("used_memory_peak_human")
	t.result["UsedMemoryLuaHuman"] = t.Get("used_memory_lua_human")
	t.result["MaxMemoryHuman"] = t.Get("maxmemory_human")
	t.result["MaxMemoryPolicy"] = t.Get("maxmemory_policy")

	// statistics info
	t.result["TotalConnectionsReceived"] = t.Get("total_connections_received")
	t.result["TotalCommandsProcessed"] = t.Get("total_commands_processed")
	t.result["InstantaneousOpsPerSec"] = t.Get("instantaneous_ops_per_sec")
	t.result["TotalNetInputBytes"] = t.Get("total_net_input_bytes")
	t.result["TotalNetOutputBytes"] = t.Get("total_net_output_bytes")
	t.result["RejectedConnections"] = t.Get("rejected_connections")
	t.result["ExpiredKeys"] = t.Get("expired_keys")
	t.result["EvictedKeys"] = t.Get("evicted_keys")
	t.result["KeyspaceHits"] = t.Get("keyspace_hits")
	t.result["KeyspaceMisses"] = t.Get("keyspace_misses")
	t.result["PubSubChannels"] = t.Get("pubsub_channels")
	t.result["PubSubPatterns"] = t.Get("pubsub_patterns")
	return nil
}

func (t *DBTask) GetName() string {
	return "数据库"
}

func (t *DBTask) Run() error {
	if err := t.GetRDSInfo(); err != nil {
		return err
	}
	if err := t.GetRedisInfo(); err != nil {
		return err
	}
	return nil
}

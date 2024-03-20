package task

import (
    "fmt"
    "inspect/pkg/common"
    "strings"

    "github.com/go-redis/redis"
)

type RedisTask struct {
    Task

    Info   map[string]string
    client *redis.Client
}

func (t *RedisTask) Init(opts *Options) error {
    t.Options = opts
    t.result = make(map[string]interface{})
    t.client = opts.GetRedisClient()
    return nil
}

func (t *RedisTask) Get(key string) string {
    if v, exist := t.Info[key]; exist {
        return v
    } else {
        return common.Empty
    }
}

func (t *RedisTask) SetServiceInfo() {
    t.result["redis_version"] = t.Get("redis_version")
    t.result["redis_mode"] = t.Get("redis_mode")
    t.result["redis_port"] = t.Get("tcp_port")
    t.result["redis_uptime"] = t.Get("uptime_in_days")
}

func (t *RedisTask) SetClientInfo() {
    t.result["redis_connect"] = t.Get("connected_clients")
    t.result["redis_cluster_connect"] = t.Get("cluster_connections")
    t.result["redis_max_connect"] = t.Get("maxclients")
    t.result["redis_blocked_connect"] = t.Get("blocked_clients")
}

func (t *RedisTask) SetMemoryInfo() {
    t.result["used_memory_human"] = t.Get("used_memory_human")
    t.result["used_memory_rss_human"] = t.Get("used_memory_rss_human")
    t.result["used_memory_peak_human"] = t.Get("used_memory_peak_human")
    t.result["used_memory_lua_human"] = t.Get("used_memory_lua_human")
    t.result["maxmemory_human"] = t.Get("maxmemory_human")
    t.result["maxmemory_policy"] = t.Get("maxmemory_policy")
}

func (t *RedisTask) SetStatisticsInfo() {
    t.result["total_connections_received"] = t.Get("total_connections_received")
    t.result["total_commands_processed"] = t.Get("total_commands_processed")
    t.result["instantaneous_ops_per_sec"] = t.Get("instantaneous_ops_per_sec")
    t.result["total_net_input_bytes"] = t.Get("total_net_input_bytes")
    t.result["total_net_output_bytes"] = t.Get("total_net_output_bytes")
    t.result["rejected_connections"] = t.Get("rejected_connections")
    t.result["expired_keys"] = t.Get("expired_keys")
    t.result["evicted_keys"] = t.Get("evicted_keys")
    t.result["keyspace_hits"] = t.Get("keyspace_hits")
    t.result["keyspace_misses"] = t.Get("keyspace_misses")
    t.result["pubsub_channels"] = t.Get("pubsub_channels")
    t.result["pubsub_patterns"] = t.Get("pubsub_patterns")
}

func (t *RedisTask) GetRedisInfo() error {
    err := t.client.Ping().Err()
    if err != nil {
        return fmt.Errorf("连接 Redis 数据库失败: %s", err)
    }

    infoStr, err := t.client.Info().Result()
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
    t.Info = info

    t.SetServiceInfo()
    t.SetClientInfo()
    t.SetMemoryInfo()
    t.SetStatisticsInfo()
    return nil
}

func (t *RedisTask) GetName() string {
    return "堡垒机后端 Redis 检查"
}

func (t *RedisTask) Run() error {
    if err := t.GetRedisInfo(); err != nil {
        return err
    }
    return nil
}

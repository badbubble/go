package mq

import "github.com/hibiken/asynq"

const redisAddr = "127.0.0.1:6379"

var MqClient *asynq.Client

func Init() {
	MqClient = asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
}

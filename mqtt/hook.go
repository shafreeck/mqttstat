package mqtt

type Hook struct {
	ConnectHook   func() error
	SubscribeHook func(topics []string, qoss []int) error
	PublishHook   func(topic string, message []byte, qos int)
}

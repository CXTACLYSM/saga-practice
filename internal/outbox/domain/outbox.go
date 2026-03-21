package domain

import "time"

type Outbox struct {
	SequenceId string
	Id         string
	Exchange   string
	RoutingKey string
	Payload    string
	CreatedAt  time.Time
}

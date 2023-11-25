package mq

import (
	"encoding/json"
	"github.com/hibiken/asynq"
)

const (
	TypeParseCode = "front:parse"
)

type CodeLocation int

const (
	LOCAL CodeLocation = iota
	REMOTE
)

type ParsePayload struct {
	// ID job ID
	ID       string
	Location CodeLocation
	Paths    []byte
	Data     []byte
}

func NewParseTask(id string, location CodeLocation, paths []byte, data []byte) (*asynq.Task, error) {
	payload, err := json.Marshal(ParsePayload{ID: id, Location: location, Paths: paths, Data: data})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeParseCode, payload), nil
}

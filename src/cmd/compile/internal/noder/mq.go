package noder

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/syntax"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hibiken/asynq"
	"log"
	"os"
	"runtime"
)

const redisAddr = "192.168.31.147:6379"

var Client *asynq.Client
var Server *asynq.Server

func WriteLog(msg string) {
	f, err := os.OpenFile("text.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}

	defer f.Close()
	if _, err := f.WriteString(msg + "\n"); err != nil {
		log.Println(err)
	}
}
func CreateClient() {
	Client = asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
}

func Run() {
	Server = asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			// Specify how many concurrent workers to use
			Concurrency: 1,
			// Optionally specify multiple queues with different priority.
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			// See the godoc for other configuration options
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeParseCode, ParseTaskHandler)
	if err := Server.Run(mux); err != nil {
		log.Fatalf("could not run server: %v", err)
	}
}

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
	Paths    []string
	Data     []byte
}

func NewParseTask(id string, location CodeLocation, paths []string, data []byte) (*asynq.Task, error) {
	payload, err := json.Marshal(ParsePayload{ID: id, Location: location, Paths: paths, Data: data})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeParseCode, payload), nil
}

type ParseTaskResult struct {
	NumLines int64
	Noders   []*noder
	PosMap   posMap
}

func ParseTaskHandler(ctx context.Context, t *asynq.Task) error {
	var p ParsePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	filenames := p.Paths
	base.Timer.Start("fe", "parse")

	// Limit the number of simultaneously open files.
	sem := make(chan struct{}, runtime.GOMAXPROCS(0)+1)

	noders := make([]*noder, len(filenames))
	for i := range noders {
		p := noder{
			err: make(chan syntax.Error),
		}
		noders[i] = &p
	}

	// Move the entire syntax processing logic into a separate goroutine to avoid blocking on the "sem".
	go func() {
		for i, filename := range filenames {
			filename := filename
			p := noders[i]
			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()
				defer close(p.err)
				fbase := syntax.NewFileBase(filename)

				f, err := os.Open(filename)
				if err != nil {
					p.error(syntax.Error{Msg: err.Error()})
					return
				}
				defer f.Close()
				// WriteLog(fmt.Sprintf("MY %v %v %v %v %v", fbase, f, p.error, p.pragma, syntax.CheckBranches))
				p.file, _ = syntax.Parse(fbase, f, p.error, p.pragma, syntax.CheckBranches) // errors are tracked via p.error
			}()
		}
	}()

	var lines uint
	var m posMap
	for _, p := range noders {
		for e := range p.err {
			base.ErrorfAt(m.makeXPos(e.Pos), 0, "%s", e.Msg)
		}
		if p.file == nil {
			base.ErrorExit()
		}
		lines += p.file.EOF.Line()

	}

	//result := &ParseTaskResult{
	//	NumLines: int64(lines),
	//	Noders:   noders,
	//	PosMap:   m,
	//}

	return nil
}

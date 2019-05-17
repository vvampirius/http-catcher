package githubWebHook

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"
)


type Payload struct {
	Repository struct {
		Name string
		Full_name string
	}
}

type Repository struct {
	Mutex sync.Mutex
	LastUpdate time.Time
}

type Hook struct {
	Repositories map[string]*Repository
	ExecutorsPath string
}

func (hook *Hook) Handler(r *http.Request) bool {
	if r.Header.Get(`Content-Type`) != `application/json` { return false }
	event := r.Header.Get(`X-Github-Event`)
	if event == `ping` || event == `push` {
		payload, err := hook.parsePayload(r.Body)
		if err != nil {
			fmt.Println(err)
			return false
		}
		fmt.Printf("%s event received for repo %s\n", event, payload.Repository.Full_name)
		//if event == `ping` { return true }
		go hook.Update(payload.Repository.Full_name)
		return true
	}
	fmt.Printf("Unknown event '%s' received\n", event)
	return false
}

func (hook *Hook) parsePayload(f io.Reader) (*Payload, error) {
	payload := new(Payload)
	j := json.NewDecoder(f)
	if err := j.Decode(payload); err != nil { return nil, err }
	return payload, nil
}

func (hook *Hook) Update(name string) {
	hookTimestamp := time.Now()
	fmt.Printf("Update %s\n", name)
	repository, ok := hook.Repositories[name]
	if !ok {
		repository = new(Repository)
		hook.Repositories[name] = repository
	}
	repository.Mutex.Lock()
	defer repository.Mutex.Unlock()
	if repository.LastUpdate.After(hookTimestamp) {
		fmt.Printf("Last Update: '%v', Hook Timestamp: '%v'\n", repository.LastUpdate, hookTimestamp)
		return
	}
	executerPath := path.Join(hook.ExecutorsPath, name)
	if _, err := os.Stat(executerPath); os.IsNotExist(err) {
		fmt.Printf("%s doesn't exist\n", executerPath)
		return
	}

	cmd := exec.Command(executerPath)
	if err := cmd.Run(); err != nil {
		fmt.Println(err)
	}

	repository.LastUpdate = time.Now()
}

func NewHook(path string) *Hook {
	o := Hook{
		Repositories: make(map[string]*Repository),
		ExecutorsPath: path,
	}
	return &o
}
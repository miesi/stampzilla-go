package notification

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/stampzilla/stampzilla-go/nodes/stampzilla-server/models/notification/email"
	"github.com/stampzilla/stampzilla-go/nodes/stampzilla-server/models/notification/file"
	"github.com/stampzilla/stampzilla-go/nodes/stampzilla-server/models/notification/nx"
	"github.com/stampzilla/stampzilla-go/nodes/stampzilla-server/models/notification/pushbullet"
	"github.com/stampzilla/stampzilla-go/nodes/stampzilla-server/models/notification/webhook"
	"github.com/stampzilla/stampzilla-go/nodes/stampzilla-server/models/notification/wirepusher"
)

type Sender struct {
	UUID       string          `json:"uuid"`
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	Parameters json.RawMessage `json:"parameters"`
}

type SenderInterface interface {
	Trigger([]string, string) error
	Release([]string, string) error
	Destinations() (map[string]string, error)
}

func (s Sender) Trigger(dest *Destination, body string) error {
	sd := NewSender(s.Type, s.Parameters)
	if sd != nil {
		return sd.Trigger(dest.Destinations, body)
	}

	return fmt.Errorf("Trigger - Not implemented")
}
func (s Sender) Release(dest *Destination, body string) error {
	sd := NewSender(s.Type, s.Parameters)
	if sd != nil {
		return sd.Release(dest.Destinations, body)
	}

	return fmt.Errorf("Release - Not implemented")
}

func (s Sender) Destinations() (map[string]string, error) {
	sd := NewSender(s.Type, s.Parameters)
	if sd != nil {
		return sd.Destinations()
	}

	return nil, fmt.Errorf("Trigger - Not implemented")
}

func NewSender(t string, p json.RawMessage) SenderInterface {
	switch t {
	case "file":
		return file.New(p)
	case "email":
		return email.New(p)
	case "webhook":
		return webhook.New(p)
	case "pushbullet":
		return pushbullet.New(p)
	case "nx":
		return nx.New(p)
	case "wirepusher":
		return wirepusher.New(p)
	}

	return nil
}

type Senders struct {
	senders map[string]Sender
	sync.RWMutex
}

func NewSenders() *Senders {
	return &Senders{
		senders: make(map[string]Sender),
	}
}

// Save saves the rules to rules.json
func (s *Senders) Save(filename string) error {
	configFile, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("senders: error saving %s: %s", filename, err.Error())
	}
	encoder := json.NewEncoder(configFile)
	encoder.SetIndent("", "\t")
	s.Lock()
	defer s.Unlock()
	err = encoder.Encode(s.senders)
	if err != nil {
		return fmt.Errorf("senders: error encoding %s: %s", filename, err.Error())
	}
	return nil
}

//Load loads the rules from rules.json
func (s *Senders) Load(filename string) error {
	logrus.Debugf("senders: loading from %s", filename)
	configFile, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Warn(err)
			return nil // We dont want to error our if the file does not exist when we start the server
		}
		return fmt.Errorf("senders: error loading %s: %s", filename, err.Error())
	}

	s.Lock()
	defer s.Unlock()
	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(&s.senders); err != nil {
		return fmt.Errorf("logic: error loading %s: %s", filename, err.Error())
	}

	return nil
}

func (s *Senders) Add(sender Sender) {
	s.Lock()
	s.senders[sender.UUID] = sender
	s.Unlock()
}

func (s *Senders) Get(id string) (Sender, bool) {
	s.RLock()
	defer s.RUnlock()
	sender, ok := s.senders[id]
	return sender, ok
}

func (s *Senders) All() map[string]Sender {
	s.RLock()
	defer s.RUnlock()
	return s.senders
}

func (s *Senders) Remove(id string) {
	s.Lock()
	delete(s.senders, id)
	s.Unlock()
}

package installer

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

type Daemon struct {
	Name      string `json:"name"`
	Autostart bool   `json:"autostart"`
}

type Config struct {
	Daemons []*Daemon `json:"daemons"`
}

func (c *Config) GetConfigForNode(name string) *Daemon {
	for _, d := range c.Daemons {
		if d.Name == name {
			return d
		}
	}
	return nil
}

func (c *Config) GetAutostartingNodes() []*Daemon {
	var daemons []*Daemon
	for _, d := range c.Daemons {
		if d.Autostart == true {
			daemons = append(daemons, d)
		}
	}
	return daemons
}

func (c *Config) GenerateDefault() error {
	nodes, err := ioutil.ReadDir("/home/stampzilla/go/src/github.com/stampzilla/stampzilla-go/nodes/")
	if err != nil {
		return err
	}

	config := &Config{}
	for _, node := range nodes {
		if !strings.Contains(node.Name(), "stampzilla-") {
			continue
		}
		name := strings.Replace(node.Name(), "stampzilla-", "", 1)
		autostart := &Daemon{Name: name}
		config.Daemons = append(config.Daemons, autostart)
	}

	*c = *config
	return nil
}

func (c *Config) SaveToFile(filepath string) error {
	configFile, err := os.Create(filepath)
	if err != nil {
		return err
	}

	var out bytes.Buffer
	b, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		return err
	}
	json.Indent(&out, b, "", "\t")
	out.WriteTo(configFile)
	return nil
}

func (c *Config) ReadConfigFromFile(filepath string) error {
	configFile, err := os.Open(filepath)
	if err != nil {
		return err
	}

	config := &Config{}
	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(&config); err != nil {
		return err
	}

	*c = *config
	return nil
}

func (c *Config) Start(what string) {
	confDir := "/etc/stampzilla/nodes/" + what
	process := NewProcess(what, confDir)
	process.Start()
}

func (c *Config) CreateConfig() {
	action := "Check config /etc/stampzilla/nodes.conf"

	if _, err := os.Stat("/etc/stampzilla/nodes.conf"); os.IsNotExist(err) {
		c.GenerateDefault()
		c.SaveToFile("/etc/stampzilla/nodes.conf")
		logrus.WithFields(logrus.Fields{
			"created": "true",
		}).Info(action)
	} else {
		logrus.WithFields(logrus.Fields{
			"exists": "true",
		}).Debug(action)
	}
}

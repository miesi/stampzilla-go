package main

import (
	"fmt"
	"math"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stampzilla/gocast"
	"github.com/stampzilla/gocast/events"
	"github.com/stampzilla/gocast/handlers"
	"github.com/stampzilla/gocast/responses"
	"github.com/stampzilla/stampzilla-go/v2/nodes/stampzilla-server/models/devices"
	"github.com/stampzilla/stampzilla-go/v2/pkg/node"
)

type Chromecast struct {
	PrimaryApp      string
	PrimaryEndpoint string
	Playing         bool
	// Paused          bool

	IsStandBy     bool
	IsIdleScreen  bool
	IsActiveInput bool

	Volume float64
	Muted  bool

	// Addr net.IP
	// Port int

	Media Chromecast_Media

	publish func(uuid string)

	mediaHandler           *handlers.Media
	mediaConnectionHandler *handlers.Connection

	appLaunch chan string

	*gocast.Device
}

type Chromecast_Media struct {
	Title    string
	SubTitle string
	Url      string
	Thumb    string
	Duration float64
}

func NewChromecast(node *node.Node, d *gocast.Device) *Chromecast {
	c := &Chromecast{
		Device:                 d,
		mediaHandler:           &handlers.Media{},
		mediaConnectionHandler: &handlers.Connection{},
		appLaunch:              make(chan string),
	}

	c.OnEvent(c.Event(node))
	return c
}

func (c *Chromecast) Play() {
	c.mediaHandler.Play()
}

func (c *Chromecast) Pause() {
	c.mediaHandler.Pause()
}

func (c *Chromecast) Stop() {
	c.mediaHandler.Stop()
}

func (c *Chromecast) PlayURL(url string, contentType string) {
	err := c.Device.ReceiverHandler.LaunchApp(gocast.AppMedia)
	if err != nil && err != handlers.ErrAppAlreadyLaunched {
		logrus.Error(err)
		return
	}

	if err != handlers.ErrAppAlreadyLaunched {
		// Wait for new media connection to launched app
		if err := c.waitForAppLaunch(gocast.AppMedia); err != nil {
			logrus.Error(err)
			return
		}
	}

	if contentType == "" {
		contentType = "audio/mpeg"
	}
	item := responses.MediaItem{
		ContentId:   url,
		StreamType:  "BUFFERED",
		ContentType: contentType,
	}
	err = c.mediaHandler.LoadMedia(item, 0, true, map[string]interface{}{})
	if err != nil {
		logrus.Error(err)
		return
	}
}

func (c *Chromecast) waitForAppLaunch(app string) error {
	delay := time.NewTimer(time.Second * 20)
	select {
	case launchedApp := <-c.appLaunch:
		if !delay.Stop() {
			<-delay.C
		}
		if app == launchedApp {
			return nil
		}
		return fmt.Errorf("Wrong app launched. Expected %s got %s", app, launchedApp)
	case <-delay.C:
		return fmt.Errorf("timeout waiting for app launch after 20 seconds")
	}
}

func (c *Chromecast) appLaunched(app string) {
	delay := time.NewTimer(time.Second * 2)
	select {
	case c.appLaunch <- app:
		if !delay.Stop() {
			<-delay.C
		}
		logrus.Debug("Notified c.appLaunch")
	case <-delay.C:
		logrus.Debug("No one is waiting for appLaunch event")
	}
}

func (c *Chromecast) Event(node *node.Node) func(event events.Event) {
	return func(event events.Event) {
		newState := make(devices.State)

		switch data := event.(type) {
		case events.Connected:
			logrus.Info(c.Name(), "- Connected")

			dev := node.GetDevice(c.Uuid())
			if dev == nil {
				dev = &devices.Device{
					Type:   "mediaplayer",
					ID:     devices.ID{ID: c.Uuid()},
					State:  devices.State{},
					Online: true,
					Name:   c.Name(),
				}
				node.AddOrUpdate(dev)
				break
			}

			if dev.Name != c.Name() {
				dev.Name = c.Name()
				node.SyncDevice(c.Uuid())
			}
			node.SetDeviceOnline(c.Uuid(), true)

			newState["playing"] = false
			newState["app"] = ""
		case events.Disconnected:
			logrus.Warn(c.Name(), "- Disconnected")

			dev := node.GetDevice(c.Uuid())
			if dev == nil {
				dev = &devices.Device{
					Type:  "mediaplayer",
					ID:    devices.ID{ID: c.Uuid()},
					State: devices.State{},
				}
				node.AddOrUpdate(dev)
				break
			}

			node.SetDeviceOnline(c.Uuid(), false)

			newState["playing"] = false
			newState["app"] = ""
		case events.AppStarted:
			logrus.Info(c.Name(), "- App started:", data.DisplayName, "(", data.AppID, ")")
			// spew.Dump("Data:", data)

			newState["app"] = data.DisplayName
			c.PrimaryApp = data.DisplayName
			c.PrimaryEndpoint = data.TransportId
			c.IsIdleScreen = data.IsIdleScreen

			c.Media.Title = data.DisplayName
			if c.IsIdleScreen {
				c.Media.Title = ""
			}
			c.Media.SubTitle = ""
			c.Media.Thumb = ""
			c.Media.Url = ""
			c.Media.Duration = 0

			// If the app supports media controls lets subscribe to it
			if data.HasNamespace("urn:x-cast:com.google.cast.media") {
				logrus.Debug(c.Name(), "- Subscribe cast.tp.connection:", data.DisplayName, "(", data.AppID, ")")
				c.Subscribe("urn:x-cast:com.google.cast.tp.connection", data.TransportId, c.mediaConnectionHandler)
				logrus.Debug(c.Name(), "- Subscribe cast.media:", data.DisplayName, "(", data.AppID, ")")
				c.Subscribe("urn:x-cast:com.google.cast.media", data.TransportId, c.mediaHandler)
			}
			c.appLaunched(data.AppID)
			logrus.Debug(c.Name(), "- Notifying appLanunched:", data.DisplayName, "(", data.AppID, ")")

		case events.AppStopped:
			logrus.Info(c.Name(), "- App stopped:", data.DisplayName, "(", data.AppID, ")")
			// spew.Dump("Data:", data)

			// unsubscribe from old channels
			for _, v := range data.Namespaces {
				c.UnsubscribeByUrnAndDestinationId(v.Name, data.TransportId)
			}
			newState["app"] = ""
			c.PrimaryApp = ""
			c.PrimaryEndpoint = ""
			newState["playing"] = false

			c.Media.Title = ""
			c.Media.SubTitle = ""
			c.Media.Thumb = ""
			c.Media.Url = ""
			c.Media.Duration = 0

		case events.ReceiverStatus:
			newState["isStandBy"] = data.Status.IsStandBy
			newState["isActiveInput"] = data.Status.IsActiveInput
			newState["volume"] = math.Round(data.Status.Volume.Level*100) / 100
			newState["muted"] = data.Status.Volume.Muted
		case events.Media:
			if data.PlayerState == "PLAYING" {
				newState["playing"] = true
			} else {
				newState["playing"] = false
			}

		default:
			logrus.Warnf("unexpected event %T: %#v\n", data, data)
		}
		node.UpdateState(c.Uuid(), newState)
	}
}

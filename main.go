package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Hundemeier/go-sacn/sacn"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"
)

var (
	Universe1 [32]byte
	fixtures  map[string]Fixture
)

type Config struct {
	Bindaddr   string    `yaml:"bindaddr"`
	Name       string    `yaml:"name"`
	Identifier string    `yaml:"identifier"`
	Universe   uint16    `yaml:"universe"`
	Mqtt       mqttconn  `yaml:"mqtt"`
	Fixtures   []Fixture `yaml:"fixtures"`
	Priority   byte      `yaml:"priority"`
}

type mqttconn struct {
	Broker        string `yaml:"broker"`
	Port          int    `yaml:"port"`
	Username      string `yaml:"username"`
	Password      string `yaml:"password"`
	Homeassistant bool   `yaml:"homeassistant"`
	Prefix        string `yaml:"homeass_prefix"`
}

type Fixture struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Address  int    `yaml:"address"`
	Value    int
	Topic    string
	MinValue int `yaml:"minValue"`
}

func main() {
	var configPath string
	var runChan = make(chan os.Signal, 1)
	signal.Notify(runChan, os.Interrupt, syscall.SIGINT)

	flag.StringVar(&configPath, "config", "./config.yml", "path to config file")
	flag.Parse()
	config, err := ReadConfig(configPath)

	// start mqtt connection
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", config.Mqtt.Broker, config.Mqtt.Port))
	opts.SetClientID(config.Identifier)

	// handle username/password here
	opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	defer client.Disconnect(250)

	// Initialize Fixtures from config
	fixtures = make(map[string]Fixture)

	for _, f := range config.Fixtures {
		fmt.Println(f.Name)
		sanitzedName := regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(f.Name, "_")
		f.Topic = config.Name + "/light/" + sanitzedName

		cmdTopic := f.Topic + "/set"
		// publish to MQTT
		client.Publish(f.Topic+"", 1, false, fmt.Sprintf("{\"state\": \"%s\", \"brightness\": %d}", "OFF", f.Value))

		// Create homeassistant discovery topic for fixture
		a := fmt.Sprintf("{\"schema\": \"json\", \"brightness\": true, \"name\": \"%s\", \"stat_t\": \"%s\", \"cmd_t\": \"%s\", \"uniq_id\": \"%s\",\"dev\": {\"identifiers\": \"%s\",\"manufacturer\": \"Bawe\",\"model\": \"mqtt2sacn\",\"name\": \"%s\",\"sw_version\": \"1.0\"}}", f.Name, f.Topic, cmdTopic, config.Identifier+"-"+sanitzedName, config.Identifier+"-"+sanitzedName, f.Name)
		fmt.Println(a)
		token := client.Publish(config.Mqtt.Prefix+"light/"+config.Name+"/"+sanitzedName+"/config", 0, true, a)
		token.Wait()

		// subscribe to set topic to receive values
		token = client.Subscribe(f.Topic+"/set", 1, nil)
		token.Wait()

		fixtures[cmdTopic] = f
	}

	// create new transmitter bound to bindaddr from config
	trans, err := sacn.NewTransmitter(config.Bindaddr, [16]byte{1, 2, 3}, config.Name)
	if err != nil {
		log.Fatal(err)
	}

	//activates the first universe
	ch, err := trans.Activate(config.Universe)
	if err != nil {
		log.Fatal(err)
	}
	//deactivate the channel on exit
	defer close(ch)

	// use multicast
	trans.SetMulticast(config.Universe, true)

	go sendUniverse(ch)
	interrupt := <-runChan
	log.Printf("Server is shutting down due to %+v\n", interrupt)

	print("End")
}

func sendUniverse(ch chan<- []byte) {
	for {
		ch <- Universe1[:]
		time.Sleep(50 * time.Millisecond)
	}
}

func ReadConfig(configPath string) (*Config, error) {
	config := &Config{}

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

type SetCmd struct {
	State      string `json:"state"`
	Brightness int    `json:"brightness"`
}

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
	if val, ok := fixtures[msg.Topic()]; ok {
		var setCmd SetCmd
		setCmd.Brightness = -1
		err := json.Unmarshal(msg.Payload(), &setCmd)
		if err != nil {
			fmt.Println("Error parsing MQTT Message as json. ")
			return
		}

		fmt.Printf(" %s Set to %d", val.Name, setCmd.Brightness)
		if setCmd.State == "OFF" {
			Universe1[val.Address-1] = 0x0
		} else if setCmd.Brightness >= 0 {
			// store and update the brightness
			val.Value = setCmd.Brightness
			fixtures[msg.Topic()] = val

			Universe1[val.Address-1] = byte(setCmd.Brightness)
			fmt.Println(Universe1)
		} else {
			// set saved value as on value
			Universe1[val.Address-1] = byte(val.Value)
			fmt.Println(Universe1)
		}
		client.Publish(val.Topic, 1, false, fmt.Sprintf("{\"state\": \"%s\", \"brightness\": %d}", setCmd.State, val.Value))
	}
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("Connected")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("Connect lost: %v", err)
}

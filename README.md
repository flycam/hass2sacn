Hass2sacn
---
Simple Daemon to control sACN Lights using homeassistant via MQTT. Currently supports one universe via multicast. 

sACN parts based on examples from 

Installation
---
Copy the binary, config and systemd-unit files. 

```
cp hass2sacn /usr/bin/
cp config.yml /etc/hass2sacn.yml
cp hass2sacn.service /etc/systemd/system/

systemctl daemon-reload
```

Start the daemon, enable if automatic start is wanted.
```
systemctl start hass2sacn
systemctl enable hass2sacn
```

Configuration
---
This is an example of a config file. For multicast, a bind address (ip of the interface in the network) is required. 

The name and 
```
---
bindaddr: "ip:5568"
name: "mqtt2sacn"
identifier: "mqtt2sacn"
mqtt:
  broker: ""
  port: 1883
  username: ""
  password: ""
  homeassistant: true
  homeass_prefix: "homeassistant/"
universe: 1
priority: 50
fixtures:
  - name: "Test wall orange"
    type: "dimmer"
    address: 19
    min_value: 10
  - name: "Test wall white"
    type: "dimmer"
    address: 21
    min_value: 10
```


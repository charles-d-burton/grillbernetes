# PiSmoker

Interfaces with a Raspberry Pi and a DS18b20 sensor to send updates to a data stream as well as provide a mechanism to control the temperatures settings.  It uses the PID algorithm to ensure the temperature stays level.  Any correctly wired DS18b20 Sensor will work with this software.  The fastest time resolution you can get is about 1 second.

### Requirements
* Go 1.12+
* Raspberry Pi
* DS18b20 (I used the MAX31850k from Adafruit)
* Relay (I used a BEM 40a SSR)
* A NATS Streaming Host to publish data to
* 

### Building

```bash
$go get -u
$go build -o pismoker
```

### Installation
Ensure you modify the `pismoker.service` file to point to your NATS Streaming host.
```bash
$cp pismoker $HOME/
$sudo cp scrips/pismoker.service /etc/systemd/system/
$sudo systemctl daemon-reload && systemctl enable pismoker && systemctl start pismoker
```

### TODO
* Handle multiple temperature sensors and take an average
* Handle multiple relays (not sure on this one)
* Integrate other sensor types such as smoke density and humidity
* Message bus abstraction to support other message bus types


package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/jeffchao/backoff"
	"github.com/paypal/gatt"
	"github.com/paypal/gatt/examples/service"
	"github.com/paypal/gatt/linux/cmd"
	"github.com/pelletier/go-toml"
)

const (
	beaconUUID           = "dc7ae043-0e31-4ba5-9d4c-75f5ae6d1b28"
	configUUID           = "b975f8d8-f42f-4842-8acd-cdb5597448fb"
	readCharacteristic   = "b50593e4-f22d-48b3-8361-9e1d22b64e2f"
	writeCharacteristic  = "2e87fb3c-7115-4a41-abc2-99d5c91d60ec"
	notifyCharacteristic = "c36f8d7a-39d7-4905-8276-d932f8db490a"

	wpaPSKConfig = `
ctrl_interface=DIR=/var/run/wpa_supplicant GROUP=netdev
update_config=1
country={{.CC}}

network={
 ssid="{{.SSID}}"
 psk="{{.Password}}"
}
	`
	deviceConfigLocation = "/etc/grillbernetes/config"
)

var (
	mc         = flag.Int("mc", 1, "Maximum concurrent connections")
	idu        = flag.Duration("idu", 0, "ibeacon duration")
	ii         = flag.Duration("ii", 5*time.Second, "ibeacon interval")
	name       = flag.String("name", "SuperSmoker3000", "Device Name")
	chmap      = flag.Int("chmap", 0x7, "Advertising channel map")
	dev        = flag.Int("dev", -1, "HCI device ID")
	chk        = flag.Bool("chk", true, "Check device LE support")
	iface      = flag.String("iface", "wlan0", "DBUS Address for WPA Supplicant")
	cc         = flag.String("cc", "US", "WPA Country Code")
	wpafile    = flag.String("wc", "/etc/wpa_supplicant/wpa_supplicant.conf", "WPA Supplicant Config File")
	remoteHost = flag.String("rh", "https://www.google.com", "Remote host to HTTP against to test network")
	dataStream = make(chan byte)
	stop       = make(chan bool, 1)
	state      = make(chan []byte, 10)
)

// cmdReadBDAddr implements cmd.CmdParam for demostrating LnxSendHCIRawCommand()
type cmdReadBDAddr struct{}

// WifiCreds store creds to setup wifi and user
type WifiCreds struct {
	CC       string `json:"cc,omitempty"`
	SSID     string `json:"ssid"`
	Password string `json:"password"`
	Username string `json:"username"`
	UID      string `json:"uid"`
}

// MachineConfig store and manipulate the configuration of the machine you're working with
type MachineConfig struct {
	Name         string
	OwnerUID     string
	DeviceSerial string
}

type status struct {
	Configured bool `json:"configured"`
}

func (c cmdReadBDAddr) Marshal(b []byte) {}
func (c cmdReadBDAddr) Opcode() int      { return 0x1009 }
func (c cmdReadBDAddr) Len() int         { return 0 }

// Get bdaddr with LnxSendHCIRawCommand() for demo purpose
func bdaddr(d gatt.Device) {
	rsp := bytes.NewBuffer(nil)
	if err := d.Option(gatt.LnxSendHCIRawCommand(&cmdReadBDAddr{}, rsp)); err != nil {
		fmt.Printf("Failed to send HCI raw command, err: %s", err)
	}
	b := rsp.Bytes()
	if b[0] != 0 {
		fmt.Printf("Failed to get bdaddr with HCI Raw command, status: %d", b[0])
	}
	log.Printf("BD Addr: %02X:%02X:%02X:%02X:%02X:%02X", b[6], b[5], b[4], b[3], b[2], b[1])
}

func startGatt() error {
	d, err := gatt.NewDevice(
		gatt.LnxMaxConnections(*mc),
		gatt.LnxDeviceID(*dev, *chk),
		gatt.LnxSetAdvertisingParameters(&cmd.LESetAdvertisingParameters{
			AdvertisingIntervalMin: 0x00f4,
			AdvertisingIntervalMax: 0x00f4,
			AdvertisingChannelMap:  0x07,
		}),
	)

	if err != nil {
		log.Printf("Failed to open device, err: %s", err)
		return err
	}

	// Register optional handlers.
	d.Handle(
		gatt.CentralConnected(func(c gatt.Central) { log.Println("Connect: ", c.ID()) }),
		gatt.CentralDisconnected(func(c gatt.Central) { log.Println("Disconnect: ", c.ID()) }),
	)

	// A mandatory handler for monitoring device state.
	onStateChanged := func(d gatt.Device, s gatt.State) {
		fmt.Printf("State: %s\n", s)
		switch s {
		case gatt.StatePoweredOn:
			// Get bdaddr with LnxSendHCIRawCommand()
			bdaddr(d)

			// Setup GAP and GATT services.
			d.AddService(service.NewGapService(*name))
			d.AddService(service.NewGattService())

			// Setup Config Service
			configService := ConfigService()
			d.AddService(configService)

			uuids := []gatt.UUID{configService.UUID()}

			// If id is zero, advertise name and services statically.
			if *idu == time.Duration(0) {
				d.AdvertiseNameAndServices(*name, uuids)
				break
			}

			// If id is non-zero, advertise name and services and iBeacon alternately.
			go func() {
				for {
					d.AdvertiseIBeacon(gatt.MustParseUUID(beaconUUID), 1, 2, -59)
					time.Sleep(*idu)

					// Advertise name and services.
					d.AdvertiseNameAndServices(*name, uuids)
					time.Sleep(*ii)
				}
			}()

		default:
		}
	}

	d.Init(onStateChanged)
	var buffer bytes.Buffer
	for {
		select {
		case datum := <-dataStream:
			if datum == 0x0A {
				err := SetupConfig(buffer.Bytes())
				//TODO: this sucks and isn't dry, I can do better
				if err != nil {
					var stat status
					stat.Configured = false
					data, err := json.Marshal(&stat)
					if err != nil {
						log.Println(err)
					}
					state <- data
					log.Println(err)
				} else {
					var stat status
					stat.Configured = true
					data, err := json.Marshal(&stat)
					if err != nil {
						log.Println(err)
					}
					state <- data
					stop <- true //Kill the loop
				}
				buffer.Reset()
				continue
			}

			buffer.WriteByte(datum)
		case <-stop:
			d.StopAdvertising()
			return nil
		}
	}
}

func ConfigService() *gatt.Service {
	s := gatt.NewService(gatt.MustParseUUID(configUUID))
	s.AddCharacteristic(gatt.MustParseUUID(readCharacteristic)).HandleReadFunc(
		func(rsp gatt.ResponseWriter, req *gatt.ReadRequest) {
			select {
			case resp := <-state:
				log.Println(string(resp))
				rsp.Write(resp)
			default:
				rsp.Write([]byte(""))
				fmt.Println("No data for caller")
			}
		})
	s.AddCharacteristic(gatt.MustParseUUID(writeCharacteristic)).HandleWriteFunc(
		func(r gatt.Request, data []byte) (status byte) {
			log.Println("Write called")
			for _, value := range data {
				dataStream <- value
			}

			return gatt.StatusSuccess
		})
	s.AddCharacteristic(gatt.MustParseUUID(notifyCharacteristic)).HandleNotifyFunc(
		func(r gatt.Request, n gatt.Notifier) {
			log.Println("Notification request")
			//TODO: This is super duper unsafe I think and I probably should do something safer
			go func() {
				for !n.Done() {
					status := <-state
					n.Write(status)
				}
			}()
		})
	return s

}

func SetupConfig(data []byte) error {
	log.Println("Config: ", string(data))

	var wificreds WifiCreds
	var machine MachineConfig
	err := json.Unmarshal(data, &wificreds)
	if err != nil {
		return err
	}
	wificreds.CC = *cc
	//Setup the machine configuration
	serial, err := GetSerial()
	if err != nil {
		return err
	}
	machine.DeviceSerial = serial
	name, err := os.Hostname()
	if err != nil {
		return err
	}
	machine.Name = strings.Replace(name, ".", "-", -1)
	machine.OwnerUID = wificreds.UID

	machineData, err := toml.Marshal(&machine)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(deviceConfigLocation, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0400)
	if err != nil {
		return err
	}
	_, err = file.Write(machineData)
	if err != nil {
		return err
	}

	//Write out the WPA config
	var tmplBytes bytes.Buffer
	wpaTemplate := template.New("WPACONFIG")
	wpaTemplate, err = wpaTemplate.Parse(wpaPSKConfig)
	if err != nil {
		return err
	}
	err = wpaTemplate.Execute(&tmplBytes, wificreds)
	if err != nil {
		return err
	}
	fmt.Println(string(tmplBytes.Bytes()))
	err = ioutil.WriteFile(*wpafile, tmplBytes.Bytes(), 0500)
	if err != nil {
		return err
	}

	cmd := exec.Command("/sbin/wpa_cli", "-i", *iface, "reconfigure")
	err = cmd.Run()
	if err != nil {
		return err
	}
	return testNetwork()
	/*fmt.Println("Scanning on iface: ", *iface)
	wifi.ScanManager.NetInterface = *iface
	bssList, err := wifi.ScanManager.Scan()
	if err != nil {
		return err
	}
	for _, bss := range bssList {
		print(bss.SSID, bss.Signal, bss.KeyMgmt)
	}
	fmt.Println("Setting up iface: ", *iface)
	wifi.ConnectManager.NetInterface = *iface
	conn, err := wifi.ConnectManager.Connect(wificreds.SSID, wificreds.Password, time.Second*60)
	if err != nil {
		return err
	}
	log.Println(conn.IP4.String())
	state <- []byte("connected")
	state <- []byte(conn.IP4.String())*/
	//return nil
}

func testNetwork() error {
	log.Println("Checking network connectivity")
	f := backoff.Fibonacci()
	f.Interval = 1 * time.Second
	f.MaxRetries = 10
	testConnectionFunc := func() error {
		_, err := http.Get(*remoteHost)
		log.Println(err)
		return err
	}
	err := f.Retry(testConnectionFunc)
	return err
}

func GetSerial() (string, error) {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		res := strings.Contains(line, "Serial")
		if res {
			serial := strings.TrimSpace(strings.Split(line, ":")[1])
			return serial, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

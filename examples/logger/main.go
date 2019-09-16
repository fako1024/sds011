package main

import (
	"flag"
	"time"

	"github.com/fako1024/sds011"
	"github.com/sirupsen/logrus"
)

var (
	devicePath string
)

func main() {

	// Parse command line parameters
	readFlags()

	// Initialize a new sds011 sensor
	sensor, err := sds011.New(devicePath)
	if err != nil {
		logrus.StandardLogger().Fatalf("Error opening %s: %s", devicePath, err)
	}

	// Ensure that device is active, then enable query mode
	if err := sensor.SetWorkMode(sds011.WorkModeActive); err != nil {
		logrus.StandardLogger().Errorf("Error setting active mode on %s: %s", devicePath, err)
	}
	if err := sensor.SetReportingMode(sds011.ReportingModeQuery); err != nil {
		logrus.StandardLogger().Errorf("Error setting query reporting mode on %s: %s", devicePath, err)
	}

	// Ensure that the sensor is put in sleep mode after termination to conserve
	// lifetime of the laser
	defer func() {
		if err := sensor.SetWorkMode(sds011.WorkModeSleep); err != nil {
			logrus.StandardLogger().Errorf("Error setting sleep mode on %s: %s", devicePath, err)
		}

		sensor.Close()
	}()

	// Continuously put the device to active mode for 10 seconds, read out the data
	// and put it back to sleep to conserve lifetime of the laser
	for {

		// Activate laser and fan, then wait for 15s for the device to settle and for
		// stable air flow
		if err := sensor.SetWorkMode(sds011.WorkModeActive); err != nil {
			logrus.StandardLogger().Errorf("Error setting active mode on %s: %s", devicePath, err)
		}
		time.Sleep(15 * time.Second)

		// Read single data point
		dataPoint, err := sensor.QueryData()
		if err != nil {
			logrus.StandardLogger().Errorf("Error reading data from %s: %s", devicePath, err)
		}

		// Log data
		logrus.StandardLogger().Infof("Read data from %s: %s", devicePath, dataPoint)

		// Put sensor to sleep mode
		if err := sensor.SetWorkMode(sds011.WorkModeSleep); err != nil {
			logrus.StandardLogger().Errorf("Error setting sleep mode on %s: %s", devicePath, err)
		}

		// Wait 5 minutes to perform the next measurement
		time.Sleep(5 * time.Minute)
	}
}

// readFlags parses command line parameters
func readFlags() {
	flag.StringVar(&devicePath, "d", "/dev/ttyUSB0", "Device / socket path to connect to")

	flag.Parse()
}

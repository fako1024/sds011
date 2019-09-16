package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/fako1024/sds011"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
)

// Health denotes the result of a health check
type Health struct {
	OK      bool
	Details string
}

var maxDataAge = time.Minute

// Simple global variables to hold configuration / data
var (
	devicePath       string
	serverEndpoint   string
	spinUpDuration   time.Duration
	measurementDelay time.Duration

	currentData *sds011.DataPoint
	health      *Health
)

func main() {

	// Parse command line parameters
	readFlags()

	// Start the echo server
	go startServer()

	// Continuously try looping / extracting data (wrapped in additional loop since
	// the device occasionally loses connection)
	for {
		readLoop()

		// Back off for ten seconds to allow device to (re-)settle
		time.Sleep(10 * time.Second)
	}
}

// readLoop continuously reads lines from the device
func readLoop() {

	// Recover from potential panic when reading from device
	defer func() {
		if r := recover(); r != nil {
			logrus.StandardLogger().Errorf("Panic recovered in readLoop(): %s", r)
			health = &Health{
				OK:      false,
				Details: fmt.Sprintf("Panic recovered in readLoop(): %s", r),
			}
		}
	}()

	// Initialize a new sds011 sensor / station
	sensor, err := sds011.New(devicePath)
	if err != nil {
		logrus.StandardLogger().Errorf("Error opening %s: %s", devicePath, err)
		health = &Health{
			OK:      false,
			Details: fmt.Sprintf("Error opening %s: %s", devicePath, err),
		}

		return
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

		// Activate laser and fan, then wait for the device to settle and for
		// stable air flow
		if err := sensor.SetWorkMode(sds011.WorkModeActive); err != nil {
			logrus.StandardLogger().Errorf("Error setting active mode on %s: %s", devicePath, err)
		}
		time.Sleep(spinUpDuration)

		// Read single data point
		dataPoint, err := sensor.QueryData()
		if err != nil {
			logrus.StandardLogger().Errorf("Error reading data from %s: %s", devicePath, err)
			health = &Health{
				OK:      false,
				Details: fmt.Sprintf("Error reading data from %s: %s", devicePath, err),
			}
		}

		// Put sensor to sleep mode
		if err := sensor.SetWorkMode(sds011.WorkModeSleep); err != nil {
			logrus.StandardLogger().Errorf("Error setting sleep mode on %s: %s", devicePath, err)
		}

		// Assign newly read data to current data
		currentData = dataPoint
		health = &Health{
			OK: true,
		}

		// Wait to perform the next measurement
		time.Sleep(measurementDelay)
	}
}

// readFlags parses command line parameters
func readFlags() {
	flag.StringVar(&devicePath, "d", "/dev/ttyUSB0", "Device / socket path to connect to")
	flag.StringVar(&serverEndpoint, "s", "0.0.0.0:8000", "Server endpoint to listen on")
	flag.DurationVar(&spinUpDuration, "spinUpDuration", 30*time.Second, "Time to wait for fan / air flow to settle before taking the measurement")
	flag.DurationVar(&measurementDelay, "measurementDelay", 5*time.Minute, "Time to wait between measurements")

	flag.Parse()

	maxDataAge = 2 * measurementDelay
}

// startServer launches an echo middleware to listen for data requests
func startServer() {

	// Create echo server instance
	e := echo.New()

	// Routes
	e.GET("/", returnData)
	e.GET("/health", returnHealth)

	// Start server
	logrus.StandardLogger().Fatal(e.Start(serverEndpoint))
}

// Data return handler
func returnData(c echo.Context) error {

	// If there is no data (yet), signify via HTTP error
	if currentData == nil {
		return c.String(http.StatusNoContent, "No data yet")
	}

	return c.JSONPretty(http.StatusOK, currentData, "  ")
}

// Health handler
func returnHealth(c echo.Context) error {

	// If there is no health (yet), signify via HTTP error
	if health == nil {
		return c.String(http.StatusNoContent, "No health data yet")
	}

	// If the current health is not ok, return it
	if health.OK == false {
		return c.JSONPretty(http.StatusOK, health, "  ")
	}

	// If the data is too old, return a warning
	if currentData != nil && currentData.TimeStamp.Before(time.Now().Add(-2*time.Minute)) {
		return c.JSONPretty(http.StatusOK, &Health{
			OK:      false,
			Details: fmt.Sprintf("Data is older than %v", maxDataAge),
		}, "  ")
	}

	return c.JSONPretty(http.StatusOK, health, "  ")
}

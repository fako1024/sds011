# Simple go package to control / read data from Nova Fitness SDS011 fine dust sensor
This package allows to extract structured data from an SDS011 fine dust / particle density sensor device (see [here](http://www.inovafitness.com/en/a/chanpinzhongxin/95.html) for details / specs). Usage is fairly trivial (see examples directory for a simple console logger implementation).

## Features
- Extraction of firmware version / date
- Reading / setting of reporting mode (continuous / query-based)
- Reading / setting of working mode (active / sleep)
- Polling / query of fine dust data (PM2.5 / PM10) values

## Installation
```bash
go get -u github.com/fako1024/sds011
```

## Example
```go
// Initialize a new SDS011 sensor
sensor, err := sds011.New("/dev/ttyUSB0")
if err != nil {
  logrus.StandardLogger().Fatalf("Error opening /dev/ttyUSB0: %s", err)
}

// Ensure that device is active, then enable query mode
if err := sensor.SetWorkMode(sds011.WorkModeActive); err != nil {
  logrus.StandardLogger().Errorf("Error setting active mode: %s", err)
}
if err := sensor.SetReportingMode(sds011.ReportingModeQuery); err != nil {
  logrus.StandardLogger().Errorf("Error setting query reporting mode: %s", err)
}

// Ensure that the sensor is put in sleep mode after termination to conserve
// lifetime of the laser
defer func() {
  if err := sensor.SetWorkMode(sds011.WorkModeSleep); err != nil {
    logrus.StandardLogger().Errorf("Error setting sleep mode: %s", err)
  }

  sensor.Close()
}()

// Continuously put the device to active mode for 30 seconds, read out the data
// and put it back to sleep to conserve lifetime of the laser
for {

  // Activate laser and fan, then wait for 30s for the device to settle and for
  // stable air flow
  if err := sensor.SetWorkMode(sds011.WorkModeActive); err != nil {
    logrus.StandardLogger().Errorf("Error setting active mode: %s", err)
  }
  time.Sleep(30 * time.Second)

  // Read single data point
  dataPoint, err := sensor.QueryData()
  if err != nil {
    logrus.StandardLogger().Errorf("Error reading data: %s", err)
  }

  // Log data
  logrus.StandardLogger().Infof("Read data: %s", dataPoint)

  // Put sensor to sleep mode
  if err := sensor.SetWorkMode(sds011.WorkModeSleep); err != nil {
    logrus.StandardLogger().Errorf("Error setting sleep mode: %s", err)
  }

  // Wait 5 minutes to perform the next measurement
  time.Sleep(5 * time.Minute)
}
```

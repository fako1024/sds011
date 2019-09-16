package sds011

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/jacobsa/go-serial/serial"
)

// ReportingMode wraps the device reporting mode
type ReportingMode string

// WorkMode wraps the device working mode
type WorkMode string

// Command prefixes
const (
	CommandGetFirmwarePrefix      = "aab40700"
	CommandGetWorkModePrefix      = "aab40600"
	CommandGetReportingModePrefix = "aab40200"
	CommandGetWorkPeriodPrefix    = "aab40800"

	CommandSetWorkModePrefix      = "aab40601"
	CommandSetReportingModePrefix = "aab40201"
	CommandSetWorkPeriodPrefix    = "aab40801"
)

const (

	// ReportingModeActive denotes active reporting (device sends data continuously)
	ReportingModeActive = ReportingMode("00")

	// ReportingModeQuery denotes manual reporting (data must be queried)
	ReportingModeQuery = ReportingMode("01")

	// WorkModeSleep denotes inactive mode (laser + fan powered down)
	WorkModeSleep = WorkMode("00")

	// WorkModeActive denotes active mode (laser + fan operative)
	WorkModeActive = WorkMode("01")

	// WorkPeriodContinuous denotes continuous operation of the device
	WorkPeriodContinuous = 0

	// WorkPeriodMax denotes the maximum delay between measurements (30 minutes)
	WorkPeriodMax = 30
)

// SDS011 denotes a Nova Fitness SDS011 fine dust sensor endpoint
type SDS011 struct {
	socket string
	port   io.ReadWriteCloser
}

// New creates a new SDS011 object
func New(socket string) (*SDS011, error) {

	// Define default options for SDS011 device
	defaultOptions := serial.OpenOptions{
		PortName:        socket,
		BaudRate:        9600,
		DataBits:        8,
		StopBits:        1,
		ParityMode:      serial.PARITY_NONE,
		MinimumReadSize: 1,
	}

	// Open the port
	port, err := serial.Open(defaultOptions)
	if err != nil {
		return nil, err
	}

	// Create and return new object
	return &SDS011{
		socket: socket,
		port:   port,
	}, nil
}

// Close closes the connection to the device
func (s *SDS011) Close() error {
	return s.port.Close()
}

// GetFirmware determines the firmware version of the sensor
func (s *SDS011) GetFirmware() (string, error) {
	rxData, err := s.executeCommand(CommandGetFirmwarePrefix + "0000000000000000000000ffff")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("20%d-%d-%d",
		int(rxData[3]),
		int(rxData[4]),
		int(rxData[5])), nil
}

// GetWorkMode determines the current working mode of the sensor
func (s *SDS011) GetWorkMode() (WorkMode, error) {
	rxData, err := s.executeCommand(CommandGetWorkModePrefix + "0000000000000000000000ffff")
	if err != nil {
		return "", err
	}

	return WorkMode(hex.EncodeToString([]byte{rxData[4]})), nil
}

// SetWorkMode sets the current working mode of the sensor
func (s *SDS011) SetWorkMode(mode WorkMode) error {
	rxData, err := s.executeCommand(CommandSetWorkModePrefix + string(mode) + "00000000000000000000ffff")
	if err != nil {
		return err
	}

	if confirmedMode := WorkMode(hex.EncodeToString([]byte{rxData[4]})); confirmedMode != mode {
		return fmt.Errorf("Unexpected work mode confirmation, want %s, have %s", mode, confirmedMode)
	}

	return nil
}

// GetReportingMode determines the current reporting mode of the sensor
func (s *SDS011) GetReportingMode() (ReportingMode, error) {
	rxData, err := s.executeCommand(CommandGetReportingModePrefix + "0000000000000000000000ffff")
	if err != nil {
		return "", err
	}

	return ReportingMode(hex.EncodeToString([]byte{rxData[4]})), nil
}

// SetReportingMode sets the current reporting mode of the sensor
func (s *SDS011) SetReportingMode(mode ReportingMode) error {
	rxData, err := s.executeCommand(CommandSetReportingModePrefix + string(mode) + "00000000000000000000ffff")
	if err != nil {
		return err
	}

	if confirmedMode := ReportingMode(hex.EncodeToString([]byte{rxData[4]})); confirmedMode != mode {
		return fmt.Errorf("Unexpected reporting mode confirmation, want %s, have %s", mode, confirmedMode)
	}

	return nil
}

// GetWorkPeriod determines the current working period of the sensor (work for
// 30 seconds, sleep for n minutes)
func (s *SDS011) GetWorkPeriod() (int, error) {
	rxData, err := s.executeCommand(CommandGetWorkPeriodPrefix + "0000000000000000000000ffff")
	if err != nil {
		return 0, err
	}

	return int(rxData[4]), nil
}

// SetWorkPeriod sets the working period of the sensor (work for 30 seconds, sleep
// for n minutes)
// NOTE: 0 denots continuous operation
func (s *SDS011) SetWorkPeriod(delayMinutes int) error {

	if delayMinutes < WorkPeriodContinuous || delayMinutes > WorkPeriodMax {
		return fmt.Errorf("Requested working period out of limits, must be between 0 and 30 (minutes)")
	}

	rxData, err := s.executeCommand(CommandSetWorkPeriodPrefix + fmt.Sprintf("%02x", delayMinutes) + "00000000000000000000ffff")
	if err != nil {
		return err
	}

	if confirmedDelay := int(rxData[4]); confirmedDelay != delayMinutes {
		return fmt.Errorf("Unexpected working period confirmation, want %d, have %d", delayMinutes, confirmedDelay)
	}

	return nil
}

// QueryData extract the current PM2.5 and PM10 values from the sensor (in query mode)
func (s *SDS011) QueryData() (*DataPoint, error) {

	rxData, err := s.executeCommand("aab404000000000000000000000000ffff")
	if err != nil {
		return nil, err
	}

	pm25, pm10, err := decodeSensorValues(rxData[2:6])
	if err != nil {
		return nil, err
	}

	// Create & return a data point
	return &DataPoint{
		TimeStamp: time.Now(),
		PM25:      pm25,
		PM10:      pm10,
	}, nil
}

// WaitForData extract the current PM2.5 and PM10 values from the sensor (in continuous mode)
// Data is returned upon reception from the serial endpoint
func (s *SDS011) WaitForData() (*DataPoint, error) {

	rxData, err := s.readRawData()
	if err != nil {
		return nil, err
	}

	if err = validateRxData(rxData); err != nil {
		return nil, err
	}

	pm25, pm10, err := decodeSensorValues(rxData[2:6])
	if err != nil {
		return nil, err
	}

	// Create & return a data point
	return &DataPoint{
		TimeStamp: time.Now(),
		PM25:      pm25,
		PM10:      pm10,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////

func (s *SDS011) executeCommand(hexCMD string) ([]byte, error) {

	txData, err := createCommand(hexCMD)
	if err != nil {
		return nil, err
	}

	if err := s.writeRawData(txData); err != nil {
		return nil, err
	}

	rxData, err := s.readRawData()
	if err != nil {
		return nil, err
	}

	if err = validateRxData(rxData); err != nil {
		return nil, err
	}

	return rxData, nil
}

const serialTimeout = 5 * time.Second

type serialReadResult struct {
	data []byte
	err  error
}

// readRawData extracts data from the port
func (s *SDS011) readRawData() ([]byte, error) {

	dataChannel := make(chan serialReadResult, 1)

	go func() {

		// Wrap reader around port
		reader := bufio.NewReader(s.port)

		// Read full data line until termination signal is received
		reply, err := reader.ReadBytes('\xab')

		dataChannel <- serialReadResult{
			data: reply,
			err:  err,
		}
	}()

	select {
	case res := <-dataChannel:
		return res.data, res.err
	case <-time.After(serialTimeout):
		return nil, fmt.Errorf("Timeout while reading from serial port (device in sleep mode?)")
	}
}

// writeRawData writes data to the port
func (s *SDS011) writeRawData(data []byte) error {

	n, err := s.port.Write(data)
	if err != nil {
		return err
	}

	if n != len(data) {
		return fmt.Errorf("Unexpected number of bytes written")
	}
	// Return the raw data received
	return nil
}

////////////////////////////////////////////////////////////////////////////////

func validateRxData(data []byte) error {
	if len(data) != 10 {
		return fmt.Errorf("Unexpected data length, want 10, have %d", len(data))
	}

	if calcChecksum(data[2:8]) != data[8] {
		return fmt.Errorf("Checksum mismatch")
	}

	return nil
}

func calcChecksum(data []byte) (sum byte) {
	for _, dataByte := range data {
		sum += dataByte
	}
	return
}

func createCommand(hexCMD string) ([]byte, error) {

	txData, err := hex.DecodeString(hexCMD)
	if err != nil {
		return nil, err
	}

	return append(txData, calcChecksum(txData[2:]), 171), nil
}

// decodeSensorValues extracts the floating-point representations of the PM2.5
// and PM10 particle densities from the raw bytes
func decodeSensorValues(rawData []byte) (float64, float64, error) {

	if len(rawData) != 4 {
		return 0., 0., fmt.Errorf("Unexpected length of raw data, need exactly 4 bytes, have %d", len(rawData))
	}

	// Convert data to count (little endian)
	var count25, count10 int16
	buf := bytes.NewBuffer(rawData[:2])
	if err := binary.Read(buf, binary.LittleEndian, &count25); err != nil {
		return 0., 0., fmt.Errorf("Error parsing PM2.5 value: %s", err)
	}
	buf = bytes.NewBuffer(rawData[2:])
	if err := binary.Read(buf, binary.LittleEndian, &count10); err != nil {
		return 0., 0., fmt.Errorf("Error parsing PM10 value: %s", err)
	}

	return 0.1 * float64(count25), 0.1 * float64(count10), nil
}

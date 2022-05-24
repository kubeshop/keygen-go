package keygen

import (
	"errors"
	"time"

	"github.com/keygen-sh/jsonapi-go"
)

type HeartbeatStatusCode string

const (
	HeartbeatStatusCodeNotStarted  HeartbeatStatusCode = "NOT_STARTED"
	HeartbeatStatusCodeAlive       HeartbeatStatusCode = "ALIVE"
	HeartbeatStatusCodeDead        HeartbeatStatusCode = "DEAD"
	HeartbeatStatusCodeResurrected HeartbeatStatusCode = "RESURRECTED"
)

var (
	ErrHeartbeatPingFailed = errors.New("machine heartbeat ping failed")
	ErrMachineNotFound     = errors.New("machine no longer exists")
)

type machine struct {
	ID          string `json:"-"`
	Type        string `json:"-"`
	Fingerprint string `json:"fingerprint"`
	Hostname    string `json:"hostname"`
	Platform    string `json:"platform"`
	Cores       int    `json:"cores"`
	LicenseID   string `json:"-"`
}

// Implement jsonapi.MarshalData interface
func (m machine) GetID() string {
	return m.ID
}

func (m machine) GetType() string {
	return "machines"
}

func (m machine) GetData() interface{} {
	return m
}

// Implement jsonapi.MarshalRelationships interface
func (m machine) GetRelationships() map[string]interface{} {
	relationships := make(map[string]interface{})

	relationships["license"] = jsonapi.ResourceObjectIdentifier{
		Type: "licenses",
		ID:   m.LicenseID,
	}

	return relationships
}

// Machine represents a Keygen machine object.
type Machine struct {
	ID                string                 `json:"-"`
	Type              string                 `json:"-"`
	Name              string                 `json:"name"`
	Fingerprint       string                 `json:"fingerprint"`
	Hostname          string                 `json:"hostname"`
	Platform          string                 `json:"platform"`
	Cores             int                    `json:"cores"`
	HeartbeatStatus   HeartbeatStatusCode    `json:"heartbeatStatus"`
	HeartbeatDuration int                    `json:"heartbeatDuration"`
	Created           time.Time              `json:"created"`
	Updated           time.Time              `json:"updated"`
	Metadata          map[string]interface{} `json:"metadata"`
	LicenseID         string                 `json:"-"`
}

// Implement jsonapi.MarshalData interface
func (m Machine) GetID() string {
	return m.ID
}

func (m Machine) GetType() string {
	return "machines"
}

func (m Machine) GetData() interface{} {
	// Transform public machine to private machine to only send a subset of attrs
	return machine{
		Fingerprint: m.Fingerprint,
		Hostname:    m.Hostname,
		Platform:    m.Platform,
		Cores:       m.Cores,
		LicenseID:   m.LicenseID,
	}
}

// Implement jsonapi.UnmarshalData interface
func (m *Machine) SetID(id string) error {
	m.ID = id
	return nil
}

func (m *Machine) SetType(t string) error {
	m.Type = t
	return nil
}

func (m *Machine) SetData(to func(target interface{}) error) error {
	return to(m)
}

// Machines represents an array of machine objects.
type Machines []Machine

// Implement jsonapi.UnmarshalData interface
func (m *Machines) SetData(to func(target interface{}) error) error {
	return to(m)
}

// Deactivate performs a machine deactivation for the current Machine. An error
// will be returned if the machine deactivation fails.
func (m *Machine) Deactivate() error {
	client := &Client{Account: Account, LicenseKey: LicenseKey, Token: Token, PublicKey: PublicKey, UserAgent: UserAgent}

	if _, err := client.Delete("machines/"+m.ID, nil, nil); err != nil {
		return err
	}

	return nil
}

// Monitor performs, on a loop, a machine hearbeat ping for the current Machine. An
// error channel will be returned, where any ping errors will be emitted. Pings are
// sent according to the machine's required heartbeat window, minus 30 seconds to
// account for any network lag.
func (m *Machine) Monitor() error {
	if err := m.ping(); err != nil {
		return err
	}

	go func() {
		t := (time.Duration(m.HeartbeatDuration) * time.Second) - (30 * time.Second)

		for range time.Tick(t) {
			if err := m.ping(); err != nil {
				panic(err)
			}
		}
	}()

	return nil
}

func (m *Machine) Checkout() (*MachineFile, error) {
	client := &Client{Account: Account, LicenseKey: LicenseKey, Token: Token, PublicKey: PublicKey, UserAgent: UserAgent}
	license := &License{}
	lic := &MachineFile{}

	if _, err := client.Get("me", nil, license); err != nil {
		return nil, err
	}

	if _, err := client.Post("machines/"+m.ID+"/actions/check-out?encrypt=1&include=license,license.entitlements", nil, lic); err != nil {
		return nil, err
	}

	// Pass license key + fingerprint as decryption secret
	lic.Secret = license.Key + m.Fingerprint

	return lic, nil
}

func (m *Machine) Spawn(pid string) (*Process, error) {
	client := &Client{Account: Account, LicenseKey: LicenseKey, Token: Token, PublicKey: PublicKey, UserAgent: UserAgent}
	params := &Process{
		Pid:       pid,
		MachineID: m.ID,
	}

	process := &Process{}
	if _, err := client.Post("processes", params, process); err != nil {
		return nil, err
	}

	if err := process.monitor(); err != nil {
		return process, err
	}

	return process, nil
}

func (m *Machine) ping() error {
	client := &Client{Account: Account, LicenseKey: LicenseKey, Token: Token, PublicKey: PublicKey, UserAgent: UserAgent}

	if _, err := client.Post("machines/"+m.ID+"/actions/ping", nil, m); err != nil {
		return err
	}

	return nil
}

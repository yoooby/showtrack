package vlc

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type VLC struct {
	Host     string // e.g. "127.0.0.1"
	Port     int    // e.g. 42069
	Password string
}

// Status queries VLC's HTTP interface for the current playback status
func (v *VLC) Status() (map[string]interface{}, error) {
	url := fmt.Sprintf("http://%s:%d/requests/status.json", v.Host, v.Port)
	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth("", v.Password) // VLC uses empty username and password

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}

package vlc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
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


func (v *VLC) AddToPlaylist(path string) error {
    url := fmt.Sprintf("http://%s:%d/requests/status.xml?command=in_enqueue&input=%s", 
        v.Host, v.Port, url.QueryEscape(path))
    
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return err
    }
    req.SetBasicAuth("", v.Password)
    
    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}

func (v *VLC) ClearPlaylist() error {
    url := fmt.Sprintf("http://%s:%d/requests/status.xml?command=pl_empty", 
        v.Host, v.Port)
    
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return err
    }
    req.SetBasicAuth("", v.Password)
    
    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}

func (v *VLC) Play() error {
    url := fmt.Sprintf("http://%s:%d/requests/status.xml?command=pl_play", 
        v.Host, v.Port)
    
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return err
    }
    req.SetBasicAuth("", v.Password)
    
    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}

func (v *VLC) Seek(seconds int) error {
    url := fmt.Sprintf("http://%s:%d/requests/status.xml?command=seek&val=%d", 
        v.Host, v.Port, seconds)
    
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return err
    }
    req.SetBasicAuth("", v.Password)
    
    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
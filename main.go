package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
)

type Device struct {
	Name string `json:"name"`
	MAC  string `json:"mac"`
}

var devices = []Device{
	{Name: "Desktop", MAC: "D8:43:AE:5F:AD:31"},
}

func wakeMAC(mac string) error {
	cleaned := strings.NewReplacer(":", "", "-", "").Replace(mac)
	macBytes, err := hex.DecodeString(cleaned)
	if err != nil || len(macBytes) != 6 {
		return fmt.Errorf("invalid MAC address: %s", mac)
	}

	packet := make([]byte, 6+6*16)
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], macBytes)
	}

	addr := &net.UDPAddr{IP: net.IPv4bcast, Port: 9}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to open UDP connection: %w", err)
	}
	defer conn.Close()

	if _, err = conn.Write(packet); err != nil {
		return fmt.Errorf("failed to send magic packet: %w", err)
	}
	return nil
}

func jsonResp(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello from nosleep!")
	})

	http.HandleFunc("/invoke", func(w http.ResponseWriter, r *http.Request) {
		log.Println("invoked!")
		fmt.Fprint(w, "invoked!")
	})

	http.HandleFunc("/devices", func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, http.StatusOK, devices)
	})

	http.HandleFunc("/wake", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("device")

		// API mode: ?device=X → send WOL and return JSON
		if name != "" {
			var found *Device
			for _, d := range devices {
				if strings.EqualFold(d.Name, name) {
					found = &d
					break
				}
			}
			if found == nil {
				names := make([]string, len(devices))
				for i, d := range devices {
					names[i] = d.Name
				}
				jsonResp(w, http.StatusNotFound, map[string]any{
					"error":   fmt.Sprintf("Unknown device: %q", name),
					"devices": names,
				})
				return
			}

			info := map[string]any{
				"device": found.Name,
				"mac":    found.MAC,
			}

			if err := wakeMAC(found.MAC); err != nil {
				info["status"] = "failed"
				info["error"] = err.Error()
				log.Printf("[WOL] %s (%s) → failed: %v", found.Name, found.MAC, err)
			} else {
				info["status"] = "sent"
				log.Printf("[WOL] %s (%s) → sent", found.Name, found.MAC)
			}

			jsonResp(w, http.StatusOK, info)
			return
		}

		// UI mode: no query param → show clickable device list
		var buttons string
		for _, d := range devices {
			buttons += fmt.Sprintf(`<button onclick="wake('%s')">%s</button>`, d.Name, d.Name)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>nosleep</title>
<style>
  body { font-family: system-ui, sans-serif; max-width: 400px; margin: 4rem auto; padding: 0 1rem; }
  h1 { font-size: 1.2rem; margin-bottom: 1.5rem; }
  button { display: block; width: 100%%; padding: 0.75rem; margin-bottom: 0.5rem; font-size: 1rem; cursor: pointer; border: 1px solid #ccc; border-radius: 6px; background: #fafafa; }
  button:hover { background: #f0f0f0; }
  #result { margin-top: 1rem; padding: 0.75rem; border-radius: 6px; font-size: 0.9rem; display: none; }
  .ok { background: #d4edda; color: #155724; }
  .fail { background: #f8d7da; color: #721c24; }
</style>
</head>
<body>
  <h1>nosleep</h1>
  %s
  <div id="result"></div>
  <script>
  async function wake(device) {
    const el = document.getElementById('result');
    el.style.display = 'none';
    try {
      const res = await fetch('/wake?device=' + encodeURIComponent(device));
      const data = await res.json();
      el.textContent = data.device + ' (' + data.mac + ') → ' + data.status;
      if (data.error) el.textContent += ': ' + data.error;
      el.className = data.status === 'sent' ? 'ok' : 'fail';
    } catch (e) {
      el.textContent = 'request failed: ' + e.message;
      el.className = 'fail';
    }
    el.style.display = 'block';
  }
  </script>
</body>
</html>`, buttons)
	})

	port := 3488
	log.Printf("Server is running on port %d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

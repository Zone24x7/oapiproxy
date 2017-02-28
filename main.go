package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// Client represents a client instance
type Client struct {
	BasePath string `json:"base_path"`
	RealKey  string `json:"real_key"`
}

// ProxyHandler provides handler for proxy endpoint
type ProxyHandler struct {
	keys map[string]Client
}

func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	appKey := r.Header.Get("X-APP_KEY")
	if appKey == "" {
		w.WriteHeader(400)
		fmt.Fprint(w, "X-APP_KEY header not provided")
		log.Println("ERROR: Request with no application key from", r.RemoteAddr)
		return
	}
	var targetPath string
	if r.URL.RawQuery == "" {
		targetPath = r.URL.Path
	} else {
		targetPath = fmt.Sprintf("%s?%s", r.URL.Path, r.URL.RawQuery)
	}
	if appInfo, ok := ph.keys[appKey]; ok {
		request, err := http.NewRequest(r.Method, fmt.Sprintf("%s/%s", appInfo.BasePath, targetPath), r.Body)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprint(w, "cannot create proxy request")
			log.Println("ERROR: Cannot create proxy request for:", r, "from", r.RemoteAddr, "with error message", err)
			return
		}
		request.Header = r.Header
		request.Header.Set("X-APP_KEY", appInfo.RealKey)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprint(w, "cannot retrieve proxy response")
			log.Println("ERROR: Cannot retrieve proxy response for:", r, "from", r.RemoteAddr, "with error message", err)
			return
		}
		for hk, hv := range response.Header {
			w.Header().Set(hk, hv[0])
			for hvvi, hvv := range hv {
				if hvvi > 0 {
					w.Header().Add(hk, hvv)
				}
			}
		}
		io.Copy(w, response.Body)
		log.Printf("Proxied [%s] for app key [%s] with real app key [%s] from [%s]", targetPath, appKey, appInfo.RealKey, r.RemoteAddr)
	} else {
		w.WriteHeader(400)
		fmt.Fprintf(w, "No application found for app key %s", appKey)
		log.Println("ERROR: Requested application not found", appKey, "from", r.RemoteAddr)
	}

}

// NewProxyHandler constructs a new proxy handler
func NewProxyHandler(config map[string]Client) (ProxyHandler, error) {
	return ProxyHandler{keys: config}, nil
}

func loadMap(file string, dest *map[string]Client) error {
	fileToLoadConfig, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fileToLoadConfig.Close()

	json.NewDecoder(fileToLoadConfig).Decode(dest)

	return nil
}

func main() {
	var portToRun string
	if len(os.Args) > 1 {
		portToRun = os.Args[1]
	} else {
		portToRun = "9080"
	}
	log.Println("Initializing oapi proxy server on port", portToRun)
	config := make(map[string]Client)
	err := loadMap("keys.json", &config)
	if err != nil {
		log.Fatal("Cannot load config file:", err)
	}
	proxyHandler, err := NewProxyHandler(config)
	if err != nil {
		log.Fatal("Cannot create proxy handler:", err)
	}
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", portToRun), &proxyHandler))
}

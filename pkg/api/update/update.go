package update

import (
	"io"
	"net/http"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	lock chan bool
)

// New is a factory function creating a new  Handler instance
func New(updateFn func(images []string), updateLock chan bool) *Handler {
	if updateLock != nil {
		lock = updateLock
	} else {
		lock = make(chan bool, 1)
		lock <- true
	}

	return &Handler{
		fn:   updateFn,
		Path: "/v1/update",
	}
}

// Handler is an API handler used for triggering container update scans.
//
// The /v1/update endpoint accepts both GET and POST requests and supports
// optional query parameters to target specific images.
//
// Usage Examples:
//
//	# Trigger update for all containers (non-blocking if update already running)
//	GET  /v1/update
//	POST /v1/update
//
//	# Trigger update for specific image(s) (blocking, waits for current update to finish)
//	GET  /v1/update?image=nginx
//	POST /v1/update?image=nginx
//	GET  /v1/update?image=nginx&image=redis
//	GET  /v1/update?image=nginx,redis
//
// Behavior:
//   - Without image parameter: Non-blocking. Skips if another update is already running.
//   - With image parameter(s): Blocking. Waits for any running update to complete, then
//     updates only the specified image(s).
//
// The image parameter can be:
//   - Single image: ?image=nginx
//   - Multiple parameters: ?image=nginx&image=redis
//   - Comma-separated: ?image=nginx,redis
//
// Note: HTTP method (GET vs POST) does not affect behavior. Both are handled identically.
type Handler struct {
	fn   func(images []string)
	Path string
}

// Handle is the actual http.Handle function doing all the heavy lifting
func (handle *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	log.Info("Updates triggered by HTTP API request.")

	_, err := io.Copy(os.Stdout, r.Body)
	if err != nil {
		log.Println(err)
		return
	}

	var images []string
	imageQueries, found := r.URL.Query()["image"]
	if found {
		for _, image := range imageQueries {
			images = append(images, strings.Split(image, ",")...)
		}

	} else {
		images = nil
	}

	if len(images) > 0 {
		chanValue := <-lock
		defer func() { lock <- chanValue }()
		handle.fn(images)
	} else {
		select {
		case chanValue := <-lock:
			defer func() { lock <- chanValue }()
			handle.fn(images)
		default:
			log.Debug("Skipped. Another update already running.")
		}
	}

}

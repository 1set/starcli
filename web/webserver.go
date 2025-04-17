package web

import (
	"fmt"
	"net/http"

	"github.com/1set/starbox"
	"github.com/1set/starlet"
	shttp "github.com/1set/starlet/lib/http"
	"go.uber.org/zap"
)

// Start starts a web server on the given port, builds and runs a Starbox instance for each request.
func Start(port uint16, builder func() *starbox.RunnerConfig) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// prepare envs
		resp := shttp.NewServerResponse()
		mac := builder().KeyValueMap(starlet.StringAnyMap{
			"request":  shttp.ConvertServerRequest(r),
			"response": resp.Struct(),
		})

		// run code
		_, err := mac.Execute()

		// handle error
		if err != nil {
			log.Warnw("fail to execute code", zap.Error(err))
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := fmt.Fprintf(w, "Runtime Error: %v", err); err != nil {
				log.Warnw("fail to write response", zap.Error(err))
			}
			return
		}

		// handle response
		if err = resp.Write(w); err != nil {
			w.Header().Add("Content-Type", "text/plain")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
		}
	})

	log.Infow("web server started", "port", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	if err != nil {
		log.Fatalw("fail to start web server", zap.Error(err))
	}
	return err
}

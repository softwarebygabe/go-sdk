package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/blend/go-sdk/configutil"

	"github.com/blend/go-sdk/graceful"
	"github.com/blend/go-sdk/logger"
	"github.com/blend/go-sdk/web"
	"github.com/blend/go-sdk/webutil"
)

// echo is the main controller.
type echo struct{}

// Register adds routes to the app.
func (e echo) Register(app *web.App) {
	app.GET("/", e.index)
	app.GET("/headers", e.headers)
	app.GET("/long/:seconds", e.long)
}

func (e echo) index(_ *web.Ctx) web.Result {
	return web.Text.Result("echo")
}

func (e echo) headers(r *web.Ctx) web.Result {
	webutil.WriteJSON(r.Response, http.StatusAccepted, r.Request.Header)
	return nil
}

func (e echo) long(r *web.Ctx) web.Result {
	seconds, err := web.IntValue(r.RouteParam("seconds"))
	if err != nil {
		return web.Text.BadRequest(err)
	}

	r.Response.WriteHeader(http.StatusOK)
	timeout := time.After(time.Duration(seconds) * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			{
				fmt.Fprintf(r.Response, "tick\n")
				r.Response.Flush()
			}
		case <-timeout:
			{
				fmt.Fprintf(r.Response, "timeout\n")
				r.Response.Flush()
				return nil
			}
		}
	}
}

var (
	flagBindAddress = flag.String("bind-addr", ":8080", "The bind address to use for the server")
	flagConfig      = flag.String("config", "config.yml", "The config file to read")
)

type config struct {
	BindAddress string
}

// Options returns the config web options.
func (c config) WebOptions() []web.Option {
	return []web.Option{
		web.OptBindAddr(c.BindAddress),
	}
}

func (c *config) Resolve() error {
	return configutil.AnyError(
		configutil.SetString(&c.BindAddress, configutil.String(*flagBindAddress), configutil.Env("BIND_ADDR"), configutil.String(c.BindAddress)),
	)
}

func main() {
	flag.Parse()

	log := logger.Prod()

	var cfg config
	if path, err := configutil.Read(&cfg, configutil.OptPaths(*flagConfig)); !configutil.IsIgnored(err) {
		log.Fatal(err)
		os.Exit(1)
	} else if err == nil {
		log.Infof("read config: %s", path)
	}

	app := web.New(cfg.WebOptions()...)
	app.Log = log
	app.Register(echo{})

	if err := graceful.Shutdown(app); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

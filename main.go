package main

import (
	"flag"
	_ "net/http/pprof"
	"os"

	"github.com/ssrlive/proxypool/config"
	"github.com/ssrlive/proxypool/pkg/geoIp"

	"github.com/ssrlive/proxypool/api"
	"github.com/ssrlive/proxypool/internal/app"
	"github.com/ssrlive/proxypool/internal/cron"
	"github.com/ssrlive/proxypool/internal/database"
	"github.com/ssrlive/proxypool/log"
)

var debugMode = false
var onceMode = false

func main() {
	var configFilePath = ""

	//go func() {
	//	http.ListenAndServe("0.0.0.0:6060", nil)
	//}()

	flag.StringVar(&configFilePath, "c", "", "path to config file: config.yaml")
	flag.BoolVar(&debugMode, "d", false, "debug output")
	flag.BoolVar(&onceMode, "once", false, "run single crawl+test cycle and exit (for CI)")
	flag.Parse()

	log.SetLevel(log.INFO)
	if debugMode {
		log.SetLevel(log.DEBUG)
		log.Debugln("=======Debug Mode=======")
	}
	if configFilePath == "" {
		configFilePath = os.Getenv("CONFIG_FILE")
	}
	if configFilePath == "" {
		configFilePath = "config.yaml"
	}

	config.SetFilePath(configFilePath)

	err := app.InitConfigAndGetters()
	if err != nil {
		log.Errorln("Configuration init error: %s", err.Error())
		panic(err)
	}

	exe, _ := os.Executable()
	log.Infoln("Running image path: %s", exe)

	// init GeoIp db reader and map between emoji's and countries
	err = geoIp.InitGeoIpDB()
	if err != nil {
		log.Warnln("GeoIP init failed: %s", err.Error())
	}

	if onceMode {
		// CI mode: run one full crawl+test cycle, export files, then exit
		log.Infoln("Running in once mode (single cycle)...")
		app.CrawlGo()
		return
	}

	database.InitTables()

	log.Infoln("Do the first crawl...")
	go app.CrawlGo() // 抓取主程序
	go cron.Cron()   // 定时运行
	api.Run()        // Web Serve
}

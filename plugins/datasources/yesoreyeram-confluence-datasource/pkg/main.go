package main

import (
	"os"

	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/yesoreyeram/grafana-x/plugins/datasources/yesoreyeram-confluence-datasource/pkg/plugin"
)

func main() {
	// Start listening to requests sent from Grafana. This call is blocking and
	// the plugin process lives as long as Grafana keeps the connection open.
	if err := datasource.Manage("yesoreyeram-confluence-datasource", plugin.NewDatasource, datasource.ManageOpts{}); err != nil {
		log.DefaultLogger.Error(err.Error())
		os.Exit(1)
	}
}

package cmd

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	cloudeventctl "github.com/horizoncd/horizon/core/controller/cloudevent"
	"github.com/horizoncd/horizon/core/http/cloudevent"
	"github.com/horizoncd/horizon/pkg/cluster/tekton/factory"
	"github.com/horizoncd/horizon/pkg/config/server"
	"github.com/horizoncd/horizon/pkg/param"
)

func runCloudEventServer(tektonFty factory.Factory, config server.Config,
	parameter *param.Param, middlewares ...gin.HandlerFunc) {
	r := gin.Default()
	r.Use(middlewares...)

	cloudEventCtl := cloudeventctl.NewController(tektonFty, parameter)

	cloudevent.RegisterRoutes(r, cloudevent.NewAPI(cloudEventCtl))

	log.Fatal(r.Run(fmt.Sprintf(":%d", config.Port)))
}

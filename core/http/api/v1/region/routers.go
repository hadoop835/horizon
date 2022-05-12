package region

import (
	"net/http"

	"g.hz.netease.com/horizon/pkg/server/route"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes register routes
func RegisterRoutes(engine *gin.Engine, api *API) {
	apiGroup := engine.Group("/apis/core/v1")

	var routes = route.Routes{
		{
			Method:      http.MethodGet,
			Pattern:     "/regions",
			HandlerFunc: api.listRegions,
		},
	}
	route.RegisterRoutes(apiGroup, routes)
}

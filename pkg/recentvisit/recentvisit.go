package recentvisit

import (
	"github.com/gin-gonic/gin"
	"github.com/yametech/devops/pkg/api"
	"github.com/yametech/devops/pkg/common"
	"github.com/yametech/devops/pkg/core"
	"github.com/yametech/devops/pkg/resource/base"
	"github.com/yametech/devops/pkg/service"
	"github.com/yametech/devops/pkg/utils"
)

func RecentVisit(service service.IService) gin.HandlerFunc {
	return func(g *gin.Context) {
		uuid := g.Query("uuid")
		//userspace := g.Request.Header["user"]
		//user := userspace[0]
		user := ""
		page := 1
		pageSize := 10
		offset := (page - 1) * pageSize
		filter := map[string]interface{}{}
		//if user != "" {
		//	filter["spec.User"] = user
		//}
		filter["spec.User"] = user
		sort := map[string]interface{}{
			"metadata.created_time": -1,
		}
		data, err := service.ListByFilter(common.Namespace, common.RecentVisit, filter, sort, int64(offset), int64(page))
		if err != nil {
			api.ResponseError(g, err)
		}
		if data != nil {
			privateModule := &base.PrivateModule{}
			for _, v := range data {
				err := utils.UnstructuredObjectToInstanceObj(v, privateModule)
				if err != nil {
					api.ResponseError(g, err)
				}
			}
			if len(privateModule.Spec.Modules) < 6 {
				privateModule.Spec.Modules = append(privateModule.Spec.Modules, uuid)
				_, judge, err := service.Apply(common.DefaultNamespace, common.RecentVisit, privateModule.UUID, privateModule, true)
				if !judge && err != nil {
					api.ResponseError(g, err)
				}
			} else {
				privateModule.Spec.Modules = append(privateModule.Spec.Modules[1:], uuid)
				_, judge, err := service.Apply(common.DefaultNamespace, common.RecentVisit, privateModule.UUID, privateModule, true)
				if !judge && err != nil {
					api.ResponseError(g, err)
				}
			}

		} else {
			uuidSlice := make([]string, 0)
			uuidSlice = append(uuidSlice, uuid)
			me := &base.PrivateModule{
				Metadata: core.Metadata{},
				Spec: base.PrivateModuleSpec{
					User:    user,
					Modules: uuidSlice,
				},
			}

			_, err := service.Create(common.DefaultNamespace, common.RecentVisit, me)
			if err != nil {
				api.ResponseError(g, err)
			}
		}
	}
}

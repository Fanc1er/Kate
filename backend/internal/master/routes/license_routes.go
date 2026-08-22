package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/internal/master/license"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

func registerLicense(rg *gin.RouterGroup, d *Deps) {
	lic := rg.Group("/license")

	lic.GET("/machine-code", func(c *gin.Context) {
		code, source, err := d.License.MachineCode()
		if err != nil {
			response.FailCode(c, 500)
			return
		}
		response.OK(c, gin.H{"machine_code": code, "source": source})
	})

	lic.GET("/status", func(c *gin.Context) {
		response.OK(c, gin.H{
			"status":         d.License.Status(),
			"days_remaining": d.License.DaysRemaining(),
			"not_before":     d.License.NotBefore(),
			"not_after":      d.License.NotAfter(),
			"max_assets":     d.License.MaxAssets(),
			"max_workers":    d.License.MaxWorkers(),
		})
	})

	lic.POST("/import", func(c *gin.Context) {
		data, err := c.GetRawData()
		if err != nil || len(data) == 0 {
			response.FailCode(c, errs.CodeValidationFailed)
			return
		}
		status := d.License.Import(data)
		switch status {
		case license.StatusValid:
			response.OK(c, gin.H{"status": status})
		case license.StatusNotYetActive:
			response.OK(c, gin.H{"status": status, "not_before": d.License.NotBefore()})
		case license.StatusExpired:
			response.FailCode(c, errs.CodeLicenseExpired)
		case license.StatusMachineMismatch:
			response.FailCode(c, errs.CodeLicenseMachineMismatch)
		default:
			response.FailCode(c, errs.CodeLicenseInvalid)
		}
	})
}

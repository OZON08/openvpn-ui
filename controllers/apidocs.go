package controllers

// APIDocsController renders the Swagger UI embedded in a full-screen iframe.
type APIDocsController struct {
	BaseController
}

func (c *APIDocsController) NestPrepare() {
	if !c.IsLogin {
		c.Ctx.Redirect(302, c.LoginPath())
		c.StopRun()
		return
	}
	c.Data["breadcrumbs"] = &BreadCrumbs{Title: "API Docs"}
}

func (c *APIDocsController) Get() {
	c.TplName = "api-docs.html"
}

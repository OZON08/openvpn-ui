package controllers

import (
	"html/template"
	"path/filepath"

	passlib "gopkg.in/hlandau/passlib.v1"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	"github.com/beego/beego/v2/core/validation"
	"github.com/beego/beego/v2/server/web"
	"github.com/OZON08/openvpn-ui/lib"
	"github.com/OZON08/openvpn-ui/models"
	"github.com/OZON08/openvpn-ui/state"
)

type NewUser struct {
	NewLogin      string `orm:"size(64);unique" form:"NewLogin" valid:"Required;"`
	NewName       string `orm:"size(64);unique" form:"NewName" valid:"Required;"`
	NewIsAdmin    bool   `orm:"default(false)" form:"IsAdmin" valid:"Required;"`
	NewEmail      string `orm:"size(64)" form:"NewEmail" valid:"Required;Email"`
	NewPassword   string `orm:"size(32)" form:"NewPassword" valid:"Required;MinSize(6)"`
	NewRepassword string `orm:"-" form:"NewRepassword" valid:"Required"`
}

type UserCertRow struct {
	User  *models.User
	Certs []string
}

type ProfileController struct {
	BaseController
}

func (c *ProfileController) NestPrepare() {
	if !c.IsLogin {
		c.Ctx.Redirect(302, c.LoginPath())
		return
	}
	c.Data["breadcrumbs"] = &BreadCrumbs{
		Title: "Profile configuration",
	}
}

func (c *ProfileController) Get() {
	c.Data["xsrfdata"] = template.HTML(c.XSRFFormHTML())
	c.Data["profile"] = c.Userinfo
	c.TplName = "profile.html"

	if c.Userinfo.IsAdmin {
		o := orm.NewOrm()
		var users []*models.User
		if _, err := o.QueryTable("user").All(&users); err != nil {
			logs.Error("Failed to retrieve user profiles:", err)
			return
		}
		c.Data["users"] = users

		var assignments []UserCertRow
		for _, u := range users {
			certs, _ := models.CertsForUser(u.Id)
			assignments = append(assignments, UserCertRow{User: u, Certs: certs})
		}
		c.Data["userCertAssignments"] = assignments

		pkiIndex := filepath.Join(state.GlobalCfg.OVConfigPath, "pki/index.txt")
		allCerts, err := lib.ReadCerts(pkiIndex)
		if err != nil {
			logs.Error("Profile: ReadCerts error:", err)
		}
		var certNames []string
		for _, cert := range allCerts {
			if cert.Details != nil && cert.Details.Name != "server" && cert.EntryType == "V" {
				certNames = append(certNames, cert.Details.Name)
			}
		}
		c.Data["allCertNames"] = certNames
	}
}

func (c *ProfileController) Post() {
	c.TplName = "profile.html"
	c.Data["profile"] = c.Userinfo

	flash := web.NewFlash()

	user := models.User{}
	if err := c.ParseForm(&user); err != nil {
		logs.Error(err)
		flash.Error("%s", err.Error())
		flash.Store(&c.Controller)
		return
	}
	user.Login = c.Userinfo.Login
	c.Data["profile"] = user

	if vMap := validateUser(user); vMap != nil {
		c.Data["validation"] = vMap
		c.List()
		return
	}

	hash, err := passlib.Hash(user.Password)
	if err != nil {
		flash.Error("Unable to hash password")
		flash.Store(&c.Controller)
		return
	}
	c.Userinfo.Email = user.Email
	c.Userinfo.Name = user.Name
	c.Userinfo.Password = hash
	o := orm.NewOrm()
	if _, err := o.Update(c.Userinfo); err != nil {
		flash.Error("%s", err.Error())
	} else {
		flash.Success("Profile has been updated!")
	}
	flash.Store(&c.Controller)
	c.List()
}

func validateUser(user models.User) map[string]map[string]string {
	valid := validation.Validation{}
	b, err := valid.Valid(&user)
	if err != nil {
		logs.Error(err)
		return nil
	}
	if !b {
		return lib.CreateValidationMap(valid)
	}
	return nil
}

func validateNewUser(nuser NewUser) map[string]map[string]string {
	valid := validation.Validation{}
	b, err := valid.Valid(&nuser)
	if err != nil {
		logs.Error(err)
		return nil
	}
	if !b {
		return lib.CreateValidationMap(valid)
	}
	return nil
}

// @router /profile/create [Create]
func (c *ProfileController) Create() {
	c.TplName = "profile.html"
	c.Data["profile"] = c.Userinfo
	flash := web.NewFlash()
	user := models.User{
		Login:      c.GetString("NewLogin"),
		Name:       c.GetString("NewName"),
		Email:      c.GetString("NewEmail"),
		Password:   c.GetString("NewPassword"),
		Repassword: c.GetString("NewRepassword"),
		IsAdmin:    c.GetString("NewIsAdmin") == "on",
	}

	uParams := NewUser{
		NewLogin:      c.GetString("NewLogin"),
		NewName:       c.GetString("NewName"),
		NewEmail:      c.GetString("NewEmail"),
		NewPassword:   c.GetString("NewPassword"),
		NewRepassword: c.GetString("NewRepassword"),
		NewIsAdmin:    c.GetString("NewIsAdmin") == "on",
	}

	if err := c.ParseForm(&user); err != nil {
		logs.Error(err)
		return
	}

	if vMap := validateNewUser(uParams); vMap != nil {
		c.Data["validation"] = vMap
		c.List()
		return
	}

	o := orm.NewOrm()
	var existingUser models.User
	err := o.QueryTable("user").Filter("Login", user.Login).One(&existingUser)
	if err == nil {
		flash.Warning("User with login \"%s\" is already exists!", user.Login)
		flash.Store(&c.Controller)
		logs.Info("User already exists:", user.Login)
		c.List()
		return
	} else if err != orm.ErrNoRows {
		logs.Error(err)
		return
	}

	var lastUser models.User
	err1 := o.QueryTable("user").OrderBy("-id").One(&lastUser)
	if err1 == orm.ErrNoRows {
		lastUser.Id = 0
	} else if err1 != nil {
		logs.Error(err1)
		return
	}
	newUser := models.User{
		Id:       lastUser.Id + 1,
		Login:    user.Login,
		IsAdmin:  user.IsAdmin,
		Name:     user.Name,
		Email:    user.Email,
		Password: user.Password,
	}
	hash, err := passlib.Hash(newUser.Password)
	if err != nil {
		logs.Error("Unable to hash password", err)
		return
	}
	newUser.Password = hash
	if created, _, err := o.ReadOrCreate(&newUser, "Name"); err == nil {
		if created {
			logs.Info("New user with login \"" + user.Login + "\" created successfully.")
			flash.Success("New user with login \"%s\" created successfully.", user.Login)
			flash.Store(&c.Controller)
		} else {
			logs.Debug(newUser)
		}
	} else {
		logs.Error(err)
	}

	flash.Store(&c.Controller)
	c.List()
}

// @router /profile [post]
func (c *ProfileController) List() {
	o := orm.NewOrm()
	var users []*models.User
	if _, err := o.QueryTable("user").All(&users); err != nil {
		logs.Error("Failed to retrieve user profiles:", err)
		return
	}
	//logs.Info("Retrieved", len(users), "user profiles")
	c.Data["users"] = users
	c.TplName = "profile.html"
}

// @router /profile/delete/:key [get]
func (c *ProfileController) DeleteUser() {
	c.TplName = "profile.html"
	flash := web.NewFlash()
	id, err := c.GetInt(":key")
	if err != nil {
		logs.Error("Failed to get user ID:", err)
		return
	}

	o := orm.NewOrm()
	user := models.User{Id: int64(id)}

	// Read the user first to populate the Login field before deleting it from the database
	err = o.Read(&user)
	if err != nil {
		logs.Error("Failed to get user:", err)
		return
	}

	if _, err := o.Delete(&user); err != nil {
		logs.Error("Failed to delete user \""+user.Login+"\" profile:", err)
		flash.Error("Failed to delete user \"%s\" profile", user.Login)
		return
	}

	logs.Info("New user with login \""+user.Login+"\" deleted successfully. It had user ID: ", id)
	flash.Success("User \"%s\" deleted successfully.", user.Login)
	flash.Store(&c.Controller)
	c.List()
}

func (c *ProfileController) AssignCert() {
	if !c.Userinfo.IsAdmin {
		c.Ctx.Redirect(302, c.URLFor("MainController.Get"))
		return
	}
	flash := web.NewFlash()
	userID, err := c.GetInt64("userID")
	if err != nil {
		flash.Error("Invalid user ID")
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	certName := c.GetString("certName")
	if !lib.SafeNameRegex.MatchString(certName) {
		flash.Error("Invalid certificate name")
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	if err := models.AssignCert(userID, certName); err != nil {
		logs.Error("AssignCert failed:", err)
		flash.Error("Failed to assign certificate: %s", err.Error())
	} else {
		flash.Success("Certificate \"%s\" assigned successfully", certName)
	}
	flash.Store(&c.Controller)
	c.Redirect(c.URLFor("ProfileController.Get"), 302)
}

func (c *ProfileController) RemoveCert() {
	if !c.Userinfo.IsAdmin {
		c.Ctx.Redirect(302, c.URLFor("MainController.Get"))
		return
	}
	flash := web.NewFlash()
	userID, err := c.GetInt64("userID")
	if err != nil {
		flash.Error("Invalid user ID")
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	certName := c.GetString("certName")
	if !lib.SafeNameRegex.MatchString(certName) {
		flash.Error("Invalid certificate name")
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	if err := models.RemoveCert(userID, certName); err != nil {
		logs.Error("RemoveCert failed:", err)
		flash.Error("Failed to remove certificate: %s", err.Error())
	} else {
		flash.Success("Certificate \"%s\" removed", certName)
	}
	flash.Store(&c.Controller)
	c.Redirect(c.URLFor("ProfileController.Get"), 302)
}

func (c *ProfileController) TransferCert() {
	if !c.Userinfo.IsAdmin {
		c.Ctx.Redirect(302, c.URLFor("MainController.Get"))
		return
	}
	flash := web.NewFlash()
	fromUserID, err := c.GetInt64("fromUserID")
	if err != nil {
		flash.Error("Invalid source user ID")
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	toUserID, err := c.GetInt64("toUserID")
	if err != nil {
		flash.Error("Invalid target user ID")
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	certName := c.GetString("certName")
	if !lib.SafeNameRegex.MatchString(certName) {
		flash.Error("Invalid certificate name")
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	if err := models.TransferCert(fromUserID, toUserID, certName); err != nil {
		logs.Error("TransferCert failed:", err)
		flash.Error("Transfer failed: %s", err.Error())
	} else {
		flash.Success("Certificate \"%s\" transferred successfully", certName)
	}
	flash.Store(&c.Controller)
	c.Redirect(c.URLFor("ProfileController.Get"), 302)
}

func (c *ProfileController) SeedCerts() {
	if !c.Userinfo.IsAdmin {
		c.Ctx.Redirect(302, c.URLFor("MainController.Get"))
		return
	}
	flash := web.NewFlash()
	pkiIndex := filepath.Join(state.GlobalCfg.OVConfigPath, "pki/index.txt")
	allCerts, err := lib.ReadCerts(pkiIndex)
	if err != nil {
		flash.Error("Could not read PKI: %s", err.Error())
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	var certNames []string
	for _, cert := range allCerts {
		if cert.Details != nil && cert.EntryType == "V" {
			certNames = append(certNames, cert.Details.Name)
		}
	}
	n, err := models.SeedAssignmentsFromLogin(certNames)
	if err != nil {
		logs.Error("SeedCerts failed:", err)
		flash.Error("Seed failed: %s", err.Error())
	} else {
		flash.Success("Seeded %d assignment(s) from login names", n)
	}
	flash.Store(&c.Controller)
	c.Redirect(c.URLFor("ProfileController.Get"), 302)
}

// @router /profile/edit/:key [post]
func (c *ProfileController) EditUser() {
	c.TplName = "profile.html"
	flash := web.NewFlash()
	id, err := c.GetInt(":key")
	if err != nil {
		logs.Error("Failed to get user ID:", err)
		return
	}

	o := orm.NewOrm()
	user := models.User{Id: int64(id)}
	if err := o.Read(&user); err != nil {
		logs.Error("Failed to read user \""+user.Name+"\" profile:", err)
		flash.Error("Failed to read user \"%s\" profile", user.Name)
		return
	}

	// Updating form "username" and "email" fields
	username := c.GetString("name")
	email := c.GetString("email")

	if username != "" {
		user.Name = username
	}
	if email != "" {
		user.Email = email
	}

	if _, err := o.Update(&user); err != nil {
		logs.Error("Failed to update user \""+user.Name+"\" profile:", err)
		flash.Error("Failed to update user \"%s\" profile", user.Name)
		return
	}

	logs.Info("Updated user profile with ID", id)
	flash.Success("User \"%s\" updated successfully", user.Name)
	flash.Store(&c.Controller)
	c.List()
}

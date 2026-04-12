package controllers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"log"
	"os"
	"strings"
	"time"

	"github.com/beego/beego/v2/core/logs"
	"github.com/beego/beego/v2/server/web"
	"github.com/OZON08/openvpn-ui/lib"
	"github.com/OZON08/openvpn-ui/models"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	oauth2api "google.golang.org/api/oauth2/v2"
)

// Initialize OAuth2 configuration
var (
	oauthConf      *oauth2.Config
	allowedDomains []string
)

// generateOAuthState returns a cryptographically random hex string for use as the
// OAuth2 state parameter. A fresh value is generated per request and stored in the
// session, so CSRF attacks against the OAuth flow are not possible.
func generateOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func init() {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	allowedDomainsStr := os.Getenv("ALLOWED_DOMAINS")

	if clientID == "" {
		log.Println("Environment variable GOOGLE_CLIENT_ID not set")
	}
	if clientSecret == "" {
		log.Println("Environment variable GOOGLE_CLIENT_SECRET not set")
	}
	if redirectURL == "" {
		log.Println("Environment variable GOOGLE_REDIRECT_URL not set")
	}
	if allowedDomainsStr == "" {
		log.Println("Environment variable ALLOWED_DOMAINS not set")
	}
	oauthConf = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}

	if allowedDomainsStr != "" {
		allowedDomains = strings.Split(allowedDomainsStr, ",")
	} else {
		allowedDomains = []string{}
	}
}

type LoginController struct {
	BaseController
}

func (c *LoginController) Login() {
	if c.IsLogin {
		c.Ctx.Redirect(302, c.URLFor("MainController.Get"))
		return
	}

	c.TplName = "login.html"
	c.Data["xsrfdata"] = template.HTML(c.XSRFFormHTML())
	if !c.Ctx.Input.IsPost() {
		return
	}

	flash := web.NewFlash()
	login := c.GetString("login")
	password := c.GetString("password")

	authType, err := web.AppConfig.String("AuthType")
	if err != nil {
		flash.Warning("%s", err.Error())
		flash.Store(&c.Controller)
		return
	}
	user, err := lib.Authenticate(login, password, authType)

	if err != nil {
		flash.Warning("%s", err.Error())
		flash.Store(&c.Controller)
		return
	}
	user.Lastlogintime = time.Now()
	err = user.Update("Lastlogintime")
	if err != nil {
		flash.Warning("%s", err.Error())
		flash.Store(&c.Controller)
		return
	}
	flash.Success("Successfully logged in")
	flash.Store(&c.Controller)

	c.SetLogin(user)

	c.Redirect(c.URLFor("MainController.Get"), 303)
}

func (c *LoginController) Logout() {
	c.DelLogin()
	flash := web.NewFlash()
	flash.Success("Successfully logged out")
	flash.Store(&c.Controller)

	c.Ctx.Redirect(302, c.URLFor("LoginController.Login"))
}

func (c *LoginController) GoogleLogin() {
	state, err := generateOAuthState()
	if err != nil {
		c.Ctx.WriteString("Failed to generate OAuth state")
		return
	}
	c.SetSession("oauth_state", state)
	url := oauthConf.AuthCodeURL(state)
	c.Redirect(url, 302)
}

func (c *LoginController) GoogleCallback() {
	state := c.GetString("state")
	sessionState, _ := c.GetSession("oauth_state").(string)
	c.DelSession("oauth_state")
	if state == "" || sessionState == "" || state != sessionState {
		c.Ctx.WriteString("Invalid OAuth state")
		return
	}

	code := c.GetString("code")
	token, err := oauthConf.Exchange(context.Background(), code)
	if err != nil {
		logs.Error("OAuth code exchange failed: %v", err)
		c.Ctx.WriteString("Authentication failed")
		return
	}

	client := oauthConf.Client(context.Background(), token)
	service, err := oauth2api.New(client)
	if err != nil {
		logs.Error("OAuth2 service creation failed: %v", err)
		c.Ctx.WriteString("Authentication failed")
		return
	}

	userinfo, err := service.Userinfo.Get().Do()
	if err != nil {
		logs.Error("OAuth2 user info fetch failed: %v", err)
		c.Ctx.WriteString("Authentication failed")
		return
	}

	logs.Info("User Info: %+v", userinfo)

	// Check if the user's email domain is allowed
	emailParts := strings.Split(userinfo.Email, "@")
	if len(emailParts) != 2 {
		c.Ctx.WriteString("Invalid email address from OAuth provider")
		return
	}
	emailDomain := emailParts[1]
	allowed := false
	for _, domain := range allowedDomains {
		if strings.EqualFold(emailDomain, domain) {
			allowed = true
			break
		}
	}

	if !allowed {
		c.Data["error"] = "Your Email is not allowed to login"
		c.TplName = "login.html"
		c.Render()
		return
	}

	user, err := lib.GetUserByEmail(userinfo.Email)
	if err != nil {
		if err.Error() == "user not found" {
			// Create a new user if not found and set the default values
			user = &models.User{
				Email:         userinfo.Email,
				Name:          userinfo.Email, // Set the name to the email address
				Login:         userinfo.Email,
				Lastlogintime: time.Now(),
				Allowed:       true, // Set to true because authenticated with Google
			}
			err = user.Insert()
			if err != nil {
				logs.Error("OAuth2 failed to create new user: %v", err)
				c.Ctx.WriteString("Authentication failed")
				return
			}
		} else {
			logs.Error("OAuth2 error fetching user: %v", err)
			c.Ctx.WriteString("Authentication failed")
			return
		}
	} else {
		// Update existing user's allowed status, last login time, and name
		user.Allowed = true
		user.Lastlogintime = time.Now()
		user.Name = userinfo.Email // Set the name to the email address
		err = user.Update("Allowed", "Lastlogintime", "Name")
		if err != nil {
			logs.Error("OAuth2 failed to update user: %v", err)
			c.Ctx.WriteString("Authentication failed")
			return
		}
	}

	// Check if the user is allowed
	if !user.Allowed {
		c.Data["error"] = "Access denied"
		c.TplName = "login.html"
		c.Render()
		return
	}

	c.SetLogin(user)

	flash := web.NewFlash()
	flash.Success("Successfully logged in with Google")
	flash.Store(&c.Controller)

	c.Redirect(c.URLFor("MainController.Get"), 302)
}

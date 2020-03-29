/*
Package service implements a remote service abstraction.

Additional information can be found in our Developer Guide:

https://github.com/photoprism/photoprism/wiki
*/
package service

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/photoprism/photoprism/internal/event"
)

var log = event.Log
var client = &http.Client{}

type Type string

const (
	TypeWeb          Type = "web"
	TypeWebDAV       Type = "webdav"
	TypeFacebook     Type = "facebook"
	TypeTwitter      Type = "twitter"
	TypeFlickr       Type = "flickr"
	TypeGooglePhotos Type = "gphotos"
	TypeGoogleDrive  Type = "gdrive"
	TypeOneDrive     Type = "onedrive"
)

type Account struct {
	AccName    string
	AccOwner   string
	AccURL     string
	AccType    string
	AccKey     string
	AccUser    string
	AccPass    string
	AccShare   bool
	AccSync    bool
	RetryLimit uint
}

type Heuristic struct {
	ServiceType Type
	Domains     []string
	Paths       []string
	Method      string
}

var Heuristics = []Heuristic{
	{TypeFacebook, []string{"facebook.com", "www.facebook.com"}, []string{}, "GET"},
	{TypeTwitter, []string{"twitter.com"}, []string{}, "GET"},
	{TypeFlickr, []string{"flickr.com", "www.flickr.com"}, []string{}, "GET"},
	{TypeOneDrive, []string{"onedrive.live.com"}, []string{}, "GET"},
	{TypeGoogleDrive, []string{"drive.google.com"}, []string{}, "GET"},
	{TypeGooglePhotos, []string{"photos.google.com"}, []string{}, "GET"},
	{TypeWebDAV, []string{}, []string{"/", "/webdav", "/remote.php/dav/files/{user}", "/remote.php/webdav", "/dav/files/{user}", "/servlet/webdav.infostore/"}, "PROPFIND"},
	{TypeWeb, []string{}, []string{}, "GET"},
}

func HttpOk(method, rawUrl string) bool {
	req, err := http.NewRequest(method, rawUrl, nil)

	if err != nil {
		return false
	}

	if resp, err := client.Do(req); err != nil {
		return false
	} else if resp.StatusCode < 400 {
		return true
	}

	return false
}

func (h Heuristic) MatchDomain(match string) bool {
	if len(h.Domains) == 0 {
		return true
	}

	for _, m := range h.Domains {
		if m == match {
			return true
		}
	}

	return false
}

func (h Heuristic) Discover(rawUrl, user string) *url.URL {
	u, err := url.Parse(rawUrl)

	if err != nil {
		return nil
	}

	if HttpOk(h.Method, u.String()) {
		return u
	}

	for _, p := range h.Paths {
		strings.Replace(p, "{user}", user, -1)
		u.Path = p

		if HttpOk(h.Method, u.String()) {
			return u
		}
	}

	return nil
}

func Discover(rawUrl, user, pass string) (result Account, err error) {
	u, err := url.Parse(rawUrl)

	if err != nil {
		return result, err
	}

	u.Host = strings.ToLower(u.Host)

	result.AccUser = u.User.Username()
	result.AccPass, _ = u.User.Password()

	// Extract user info
	if user != "" {
		result.AccUser = user
	}

	if pass != "" {
		result.AccPass = pass
	}

	if user != "" || pass != "" {
		u.User = url.UserPassword(result.AccUser, result.AccPass)
	}

	// Set default scheme
	if u.Scheme == "" {
		u.Scheme = "https"
	}

	for _, h := range Heuristics {
		if !h.MatchDomain(u.Host) {
			continue
		}

		if serviceUrl := h.Discover(u.String(), result.AccUser); serviceUrl != nil {
			serviceUrl.User = nil
			result.AccURL = serviceUrl.String()
			result.RetryLimit = 3
			result.AccName = serviceUrl.Host
			result.AccType = string(h.ServiceType)

			return result, nil
		}
	}

	return result, fmt.Errorf("no supported service found at %s", rawUrl)
}
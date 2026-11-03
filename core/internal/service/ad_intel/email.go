package ad_intel

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)

// Addresses that show up in page source but are never a business inbox.
var ignoredEmailParts = []string{"example.", "sentry", "wixpress", "domain.com", "email.com", "yourname", "@2x", "godaddy"}

var ignoredEmailSuffixes = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".js", ".css"}

// contactPaths are tried after the home page.
var contactPaths = []string{"", "/contact", "/contact-us", "/about"}

// lookupIP is a variable so tests can resolve hosts without DNS.
var lookupIP = net.DefaultResolver.LookupIP

// websiteClient refuses to connect to private or local addresses, so an ad's link
// cannot be used to reach BillionMail's internal services.
var websiteClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := lookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if isPublicIP(ip) {
					return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				}
			}
			return nil, errors.New("website does not resolve to a public address")
		},
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		return nil
	},
}

func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return false
	}
	// The Docker network (172.66.1.0/24 by default) is not in a private range, so block every
	// network this server is attached to as well
	for _, n := range localNetworks() {
		if n.Contains(ip) {
			return false
		}
	}
	return true
}

var (
	localNetsOnce sync.Once
	localNets     []*net.IPNet
)

// localNetworks returns the networks of this host's interfaces; a variable so tests can replace it.
var localNetworks = func() []*net.IPNet {
	localNetsOnce.Do(func() {
		addrs, _ := net.InterfaceAddrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok {
				localNets = append(localNets, n)
			}
		}
	})
	return localNets
}

// FindEmail looks for a contact email on the advertiser's website (home and contact pages).
// Addresses on the website's own domain are preferred.
func FindEmail(ctx context.Context, website string) (string, error) {
	base, err := url.Parse(website)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return "", errors.New("no website to look up")
	}
	domain := strings.TrimPrefix(strings.ToLower(base.Hostname()), "www.")

	var fallback string
	for _, path := range contactPaths {
		page := base.Scheme + "://" + base.Host + path
		for _, email := range emailsOnPage(ctx, page) {
			if strings.HasSuffix(email, "@"+domain) || strings.HasSuffix(email, "."+domain) {
				return email, nil
			}
			if fallback == "" {
				fallback = email
			}
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", errors.New("no email found on the website")
}

func emailsOnPage(ctx context.Context, page string) []string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, page, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; BillionMail contact finder)")
	resp, err := websiteClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	return ExtractEmails(string(body))
}

// ExtractEmails returns the unique, plausible email addresses in an HTML page.
func ExtractEmails(html string) []string {
	html = strings.ReplaceAll(html, "&#64;", "@")
	html = strings.ReplaceAll(html, "%40", "@")

	seen := map[string]bool{}
	var out []string
	for _, m := range emailPattern.FindAllString(html, -1) {
		email := strings.ToLower(strings.Trim(m, ".-"))
		if seen[email] || !plausibleEmail(email) {
			continue
		}
		seen[email] = true
		out = append(out, email)
	}
	return out
}

func plausibleEmail(email string) bool {
	for _, part := range ignoredEmailParts {
		if strings.Contains(email, part) {
			return false
		}
	}
	for _, suffix := range ignoredEmailSuffixes {
		if strings.HasSuffix(email, suffix) {
			return false
		}
	}
	return true
}

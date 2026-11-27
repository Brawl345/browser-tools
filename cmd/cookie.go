package cmd

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Cookie(ctx context.Context, variant string, port int, args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "set":
			cookieSet(ctx, variant, port, args[1:])
			return
		case "delete":
			cookieDelete(ctx, variant, port, args[1:])
			return
		}
	}

	fs := flag.NewFlagSet("cookie", flag.ExitOnError)
	all := fs.Bool("all", false, "list cookies from all origins (not just the current tab's URL)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools cookie [--all]")
		fmt.Fprintln(os.Stderr, "       browser-tools cookie set <name> <value> [options]")
		fmt.Fprintln(os.Stderr, "       browser-tools cookie delete <name> [--domain <d>] [--path <p>]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	tabCtx := MustTab(MustSession(ctx, variant, port))

	var cookies []*network.Cookie
	if err := RunBounded(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		if *all {
			cookies, err = storage.GetCookies().Do(ctx)
		} else {
			var currentURL string
			if err = chromedp.Location(&currentURL).Do(ctx); err != nil {
				return err
			}
			cookies, err = network.GetCookies().WithURLs([]string{currentURL}).Do(ctx)
		}
		return err
	})); err != nil {
		fail("%v", err)
	}

	if len(cookies) == 0 {
		fmt.Println("No cookies found.")
		return
	}

	for i, c := range cookies {
		if i > 0 {
			fmt.Println()
		}
		printCookie(c)
	}

	fmt.Fprintf(os.Stderr, "\n%s%d cookie(s)%s\n", ansiDim, len(cookies), ansiReset)
}

func printCookie(c *network.Cookie) {
	fmt.Printf("%s%s%s\n", ansiBold, c.Name, ansiReset)
	fmt.Printf("  Value    : %s\n", c.Value)
	fmt.Printf("  Domain   : %s\n", c.Domain)
	fmt.Printf("  Path     : %s\n", c.Path)

	if c.Session || c.Expires < 0 {
		fmt.Printf("  Expires  : (session)\n")
	} else {
		exp := time.Unix(int64(math.Round(c.Expires)), 0)
		fmt.Printf("  Expires  : %s\n", exp.UTC().Format(time.RFC1123))
	}

	fmt.Printf("  Secure   : %v\n", c.Secure)
	fmt.Printf("  HttpOnly : %v\n", c.HTTPOnly)

	if c.SameSite != "" {
		fmt.Printf("  SameSite : %s\n", c.SameSite)
	}
}

// pageURL returns the current tab's URL when cookies can belong to it.
func pageURL(tabCtx context.Context) string {
	var u string
	if err := RunBounded(tabCtx, chromedp.Location(&u)); err != nil {
		fail("%v", err)
	}
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	return ""
}

func allCookies(tabCtx context.Context) []*network.Cookie {
	var cookies []*network.Cookie
	if err := RunBounded(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		cookies, err = storage.GetCookies().Do(ctx)
		return err
	})); err != nil {
		fail("%v", err)
	}
	return cookies
}

func urlCookies(tabCtx context.Context, u string) []*network.Cookie {
	var cookies []*network.Cookie
	if err := RunBounded(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		cookies, err = network.GetCookies().WithURLs([]string{u}).Do(ctx)
		return err
	})); err != nil {
		fail("%v", err)
	}
	return cookies
}

func sameDomain(a, b string) bool {
	return strings.EqualFold(strings.TrimPrefix(a, "."), strings.TrimPrefix(b, "."))
}

// findCookie picks the cookie a set or delete refers to: by name, narrowed by
// domain and path when given, otherwise among those the current page receives.
func findCookie(tabCtx context.Context, u, name, domain, path string) []*network.Cookie {
	var pool []*network.Cookie
	if domain == "" && u != "" {
		pool = urlCookies(tabCtx, u)
	} else {
		pool = allCookies(tabCtx)
	}
	var out []*network.Cookie
	for _, c := range pool {
		if c.Name != name || (domain != "" && !sameDomain(c.Domain, domain)) || (path != "" && c.Path != path) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func cookieSet(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("cookie set", flag.ExitOnError)
	domain := fs.String("domain", "", "cookie domain, with a leading dot to include subdomains (default: the current page's host)")
	path := fs.String("path", "", "cookie path (default /)")
	expires := fs.String("expires", "", "lifetime (30m, 12h, 7d), a date (2027-01-31, RFC 3339), a Unix timestamp, or session (default session)")
	secure := fs.Bool("secure", false, "only send over HTTPS (default: on when the page is https)")
	httpOnly := fs.Bool("http-only", false, "hide the cookie from page scripts")
	sameSite := fs.String("same-site", "", "Strict, Lax or None (default: unset, which Chrome treats as Lax)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools cookie set <name> <value> [options]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Creates the cookie, or changes an existing one with that name. Attributes")
		fmt.Fprintln(os.Stderr, "that are not passed keep their current value.")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() != 2 {
		fs.Usage()
		os.Exit(1)
	}
	name, value := fs.Arg(0), fs.Arg(1)

	var exp *cdp.TimeSinceEpoch
	if fs.Changed("expires") {
		t, err := parseExpires(*expires)
		if err != nil {
			fail("--expires: %v", err)
		}
		exp = t
	}
	var site network.CookieSameSite
	if fs.Changed("same-site") {
		switch strings.ToLower(*sameSite) {
		case "strict":
			site = network.CookieSameSiteStrict
		case "lax":
			site = network.CookieSameSiteLax
		case "none":
			site = network.CookieSameSiteNone
		default:
			fail("--same-site must be Strict, Lax or None")
		}
	}

	tabCtx := MustTab(MustSession(ctx, variant, port))
	u := pageURL(tabCtx)
	if u == "" && *domain == "" {
		fail("the current tab has no http(s) page; open one or pass --domain")
	}

	p := network.SetCookie(name, value)
	matches := findCookie(tabCtx, u, name, *domain, *path)
	if len(matches) > 1 {
		fail("%d cookies are named %s; pick one with --domain and --path (see `browser-tools cookie --all`)", len(matches), name)
	}

	cookieDomain, cookiePath := *domain, *path
	wantSecure := strings.HasPrefix(u, "https://")
	if len(matches) == 1 {
		old := matches[0]
		cookieDomain = firstNonEmpty(cookieDomain, old.Domain)
		cookiePath = firstNonEmpty(cookiePath, old.Path)
		wantSecure = old.Secure
		p = p.WithHTTPOnly(old.HTTPOnly)
		if old.SameSite != "" {
			p = p.WithSameSite(old.SameSite)
		}
		if !old.Session && old.Expires > 0 && !fs.Changed("expires") {
			t := cdp.TimeSinceEpoch(time.Unix(int64(math.Round(old.Expires)), 0))
			p = p.WithExpires(&t)
		}
	}
	cookiePath = firstNonEmpty(cookiePath, "/")
	if fs.Changed("secure") {
		wantSecure = *secure
	}
	p = p.WithSecure(wantSecure)

	if host := firstNonEmpty(cookieDomain, hostOf(u)); !strings.HasPrefix(host, ".") && (cookieDomain == "" || len(matches) == 1) {
		// Set through a URL so a host-only cookie stays host-only; Chrome makes
		// every cookie set through an https URL secure.
		scheme := "http"
		if wantSecure {
			scheme = "https"
		}
		p = p.WithURL(scheme + "://" + host + cookiePath).WithPath(cookiePath)
	} else {
		p = p.WithDomain(cookieDomain).WithPath(cookiePath)
	}
	if fs.Changed("http-only") {
		p = p.WithHTTPOnly(*httpOnly)
	}
	if site != "" {
		p = p.WithSameSite(site)
	}
	if exp != nil {
		p = p.WithExpires(exp)
	}

	if err := RunBounded(tabCtx, p); err != nil {
		fail("Chrome rejected the cookie: %v", err)
	}

	after := findCookie(tabCtx, "", name, strings.TrimPrefix(firstNonEmpty(cookieDomain, hostOf(u)), "."), cookiePath)
	if len(after) == 0 {
		fail("Chrome did not store the cookie; SameSite=None needs --secure, and __Host-/__Secure- names need --secure")
	}
	verb := "set"
	if len(matches) == 1 {
		verb = "changed"
	}
	fmt.Printf("%s✓%s %s cookie\n", ansiGreen, ansiReset, verb)
	printCookie(after[0])
}

func cookieDelete(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("cookie delete", flag.ExitOnError)
	domain := fs.String("domain", "", "only delete the cookie on this domain (default: those the current page receives)")
	path := fs.String("path", "", "only delete the cookie with this path")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools cookie delete <name> [--domain <d>] [--path <p>]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "`browser-tools clear --cookies` deletes all cookies of the current origin.")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() != 1 {
		fs.Usage()
		os.Exit(1)
	}
	name := fs.Arg(0)

	tabCtx := MustTab(MustSession(ctx, variant, port))
	u := pageURL(tabCtx)
	if u == "" && *domain == "" {
		fail("the current tab has no http(s) page; open one or pass --domain")
	}

	matches := findCookie(tabCtx, u, name, *domain, *path)
	if len(matches) == 0 {
		where := firstNonEmpty(*domain, u)
		fail("no cookie named %s for %s — `browser-tools cookie --all` lists every cookie", name, where)
	}
	if err := RunBounded(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		for _, c := range matches {
			if err := network.DeleteCookies(c.Name).WithDomain(c.Domain).WithPath(c.Path).Do(ctx); err != nil {
				return err
			}
		}
		return nil
	})); err != nil {
		fail("%v", err)
	}
	for _, c := range matches {
		fmt.Printf("%s✓%s deleted %s (%s%s)\n", ansiGreen, ansiReset, c.Name, c.Domain, c.Path)
	}
}

func hostOf(u string) string {
	rest, ok := strings.CutPrefix(u, "https://")
	if !ok {
		rest, _ = strings.CutPrefix(u, "http://")
	}
	host, _, _ := strings.Cut(rest, "/")
	host, _, _ = strings.Cut(host, "?")
	if h, _, ok := strings.Cut(host, ":"); ok && !strings.Contains(host, "]") {
		host = h
	}
	return host
}

// parseExpires reads an expiry; nil means a session cookie.
func parseExpires(s string) (*cdp.TimeSinceEpoch, error) {
	s = strings.TrimSpace(s)
	if strings.EqualFold(s, "session") {
		return nil, nil
	}
	var t time.Time
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.ParseFloat(days, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot read %q", s)
		}
		t = time.Now().Add(time.Duration(n * float64(24*time.Hour)))
	} else if d, err := time.ParseDuration(s); err == nil {
		t = time.Now().Add(d)
	} else if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		t = time.Unix(n, 0)
	} else if v, err := time.Parse(time.RFC3339, s); err == nil {
		t = v
	} else if v, err := time.ParseInLocation(time.DateOnly, s, time.Local); err == nil {
		t = v
	} else {
		return nil, fmt.Errorf("cannot read %q; use 30m, 12h, 7d, 2027-01-31, a Unix timestamp or session", s)
	}
	ts := cdp.TimeSinceEpoch(t)
	return &ts, nil
}

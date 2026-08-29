package usersvc

import (
	"regexp"
	"strconv"
	"strings"
)

var usernameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{2,31}$`)

// DeriveUsername maps Google email (+ optional name) to a User-Service username.
func DeriveUsername(email, name string) string {
	local := emailLocalPart(email)
	local = stripPlusTag(local)
	s := sanitize(local)
	if s == "" {
		s = sanitize(firstToken(name))
	}
	if s == "" {
		s = "user"
	}
	return fitUsername(s)
}

// UsernameAttempt is attempt 1 = base; later attempts append _2, _3, … (max 32 chars).
func UsernameAttempt(base string, attempt int) string {
	base = fitUsername(base)
	if attempt <= 1 {
		return base
	}
	suffix := "_" + strconv.Itoa(attempt)
	keep := 32 - len(suffix)
	if keep < 1 {
		keep = 1
	}
	stem := base
	if len(stem) > keep {
		stem = stem[:keep]
	}
	if stem == "" || !isLetter(rune(stem[0])) {
		stem = "u"
	}
	return stem + suffix
}

func emailLocalPart(email string) string {
	email = strings.TrimSpace(email)
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return email
	}
	return email[:at]
}

func stripPlusTag(local string) string {
	if i := strings.IndexByte(local, '+'); i >= 0 {
		return local[:i]
	}
	return local
}

func firstToken(name string) string {
	fields := strings.Fields(strings.TrimSpace(name))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func sanitize(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func fitUsername(s string) string {
	s = sanitize(s)
	if s == "" {
		s = "user"
	}
	if !isLetter(rune(s[0])) {
		s = "u_" + s
	}
	for len(s) < 3 {
		s += "x"
	}
	if len(s) > 32 {
		s = s[:32]
	}
	if !usernameRe.MatchString(s) {
		return "user"
	}
	return s
}

func isLetter(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

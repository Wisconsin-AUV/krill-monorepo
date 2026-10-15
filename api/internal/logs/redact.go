package logs

import "regexp"

const redacted = "[redacted]"

// secretKey matches attribute names whose values are secrets whatever they
// look like.
var secretKey = regexp.MustCompile(`(?i)(passw(or)?d|secret|token|api_?key|access_?key|private_?key|authorization|cookie|credential|database_?url|dsn)`)

// secretText matches secrets inside any text, with the parts to keep around
// them in the first and second groups.
var secretText = []*regexp.Regexp{
	// A password in a URL, like postgres://krill:hunter2@db.
	regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://[^\s:/@]+:)[^\s@/]+(@)`),
	regexp.MustCompile(`(?i)(bearer\s+)[a-z0-9._~+/=-]+`),
	// Presigned URLs work for anyone who has them until they expire.
	regexp.MustCompile(`(?i)(X-Amz-(?:Signature|Credential|Security-Token)=)[^&\s"']+`),
	regexp.MustCompile(`(?i)((?:passw(?:or)?d|secret|token|api_?key)["']?\s*[=:]\s*["']?)[^\s&"',}]+`),
}

func redactText(s string) string {
	for _, re := range secretText {
		s = re.ReplaceAllString(s, "${1}"+redacted+"${2}")
	}
	return s
}

func redactAttr(key, value string) string {
	if value != "" && secretKey.MatchString(key) {
		return redacted
	}
	return redactText(value)
}

package guard

import "regexp"

// dangerPattern couples a compiled regular expression with the human-readable
// reason reported when a command matches it.
type dangerPattern struct {
	re     *regexp.Regexp
	reason string
}

// dangerousPatterns is the deterministic list of dangerous shell-command
// patterns defined in SPEC §3.6. Matching is case-insensitive so that
// "DROP TABLE", "drop table", "git reset --hard" etc. are all caught.
//
// Matching is intentionally conservative (pattern-based, no semantic parsing):
// a command that merely quotes a dangerous fragment is also blocked, which
// favours "intercept every dangerous command, no exceptions" (§4.2).
var dangerousPatterns = []dangerPattern{
	{
		// rm -rf / rm -r / rm -fr ... and rmdir (recursive deletes).
		re:     regexp.MustCompile(`(?i)\brm\s+-\w*[rR]\w*\b|\brmdir\b`),
		reason: "recursive delete detected (rm -rf / rmdir)",
	},
	{
		// git push --force / git push -f (force push).
		re:     regexp.MustCompile(`(?i)\bgit\s+push\b.*?(?:--force|-f)\b`),
		reason: "force push detected (git push --force)",
	},
	{
		// git reset --hard (discards working-tree changes).
		re:     regexp.MustCompile(`(?i)\bgit\s+reset\b.*?--hard\b`),
		reason: "hard reset detected (git reset --hard)",
	},
	{
		// git clean (deletes untracked files).
		re:     regexp.MustCompile(`(?i)\bgit\s+clean\b`),
		reason: "git clean detected",
	},
	{
		// Destructive database operations: DROP <object> / DELETE FROM.
		re:     regexp.MustCompile(`(?i)\b(?:drop\s+(?:table|database|schema|index|view|trigger|sequence)\b|delete\s+from\b)`),
		reason: "destructive database operation detected (DROP / DELETE)",
	},
	{
		// Redirect output into a device file, e.g. > /dev/sda, 2>/dev/null and
		// the no-space form cmd>/dev/sda.
		re:     regexp.MustCompile(`(?i)(?:^|[\w\s;&|(])\d*>\s*/dev/`),
		reason: "write to device file detected (> /dev/)",
	},
	{
		// Over-permissive permissions: chmod 777 / chmod 0777.
		re:     regexp.MustCompile(`(?i)\bchmod\b.*\b0?777\b`),
		reason: "overly permissive permissions detected (chmod 777)",
	},
	{
		// curl | bash / curl | sh (pipe remote script into a shell).
		re:     regexp.MustCompile(`(?i)\bcurl\b[^|]*\|\s*(?:ba)?sh\b`),
		reason: "pipe remote script into shell detected (curl | bash)",
	},
	{
		// wget -O - | sh / wget ... | bash (pipe remote script into a shell).
		re:     regexp.MustCompile(`(?i)\bwget\b[^|]*\|\s*(?:ba)?sh\b`),
		reason: "pipe remote script into shell detected (wget | sh)",
	},
}

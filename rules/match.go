package rules

import (
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/lepinkainen/avella/config"
)

// Matches returns true if the file at path satisfies all predicates in the match rule.
// All specified predicates are AND-combined.
func Matches(path string, info os.FileInfo, rule config.MatchRule, regex *regexp.Regexp) bool {
	if regex != nil && !regex.MatchString(filepath.Base(path)) {
		return false
	}

	if rule.MinAgeSeconds > 0 {
		age := time.Since(info.ModTime())
		if age < time.Duration(rule.MinAgeSeconds)*time.Second {
			return false
		}
	}

	if rule.MinSize > 0 && info.Size() < rule.MinSize {
		return false
	}

	if rule.MaxSize > 0 && info.Size() > rule.MaxSize {
		return false
	}

	return true
}

package rules

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/lepinkainen/avella/config"
	avtemplate "github.com/lepinkainen/avella/template"
)

// Matches returns true if the file at path satisfies all predicates in the match rule.
// All specified predicates are AND-combined.
// minAge overrides rule.MinAgeSeconds when > 0 (caller pre-parses min_age string).
func Matches(path string, info os.FileInfo, rule config.MatchRule, regex *regexp.Regexp, minAge time.Duration) bool {
	if regex != nil && !regex.MatchString(filepath.Base(path)) {
		return false
	}

	if rule.FilenameGlob != "" {
		matched, _ := filepath.Match(rule.FilenameGlob, filepath.Base(path))
		if !matched {
			return false
		}
	}

	if rule.FileType != "" {
		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		if !strings.EqualFold(avtemplate.ClassifyExt(ext), rule.FileType) {
			return false
		}
	}

	effectiveMinAge := minAge
	if effectiveMinAge == 0 && rule.MinAgeSeconds > 0 {
		effectiveMinAge = time.Duration(rule.MinAgeSeconds) * time.Second
	}
	if effectiveMinAge > 0 {
		if time.Since(info.ModTime()) < effectiveMinAge {
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

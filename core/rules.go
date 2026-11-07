package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/gholamtaksir/Harp/engine"
)

// kindTypes maps engine matcher kinds to the rule types used in settings.
var kindTypes = map[engine.Kind]string{
	engine.KindIPCIDR:           "ip",
	engine.KindPrivate:          "private",
	engine.KindDomain:           "domain",
	engine.KindDomainSuffix:     "suffix",
	engine.KindDomainKeyword:    "keyword",
	engine.KindDomainRegex:      "regex",
	engine.KindProcessName:      "app",
	engine.KindProcessPath:      "path",
	engine.KindProcessPathRegex: "path_regex",
	engine.KindPort:             "port",
	engine.KindPortRange:        "port",
	engine.KindProtocol:         "protocol",
	engine.KindNetwork:          "network",
	engine.KindGeoIP:            "geoip",
	engine.KindGeoSite:          "geosite",
	engine.KindRuleSet:          "ruleset",
}

// NewRuleID returns a random rule identifier.
func NewRuleID() string {
	buffer := make([]byte, 6)
	_, _ = rand.Read(buffer)
	return hex.EncodeToString(buffer)
}

// ParseRulesText reads rules in the CLI rules file format, one
// "<action> <matcher>..." entry per line.
func ParseRulesText(text string) ([]Rule, error) {
	parsed, err := engine.ParseRulesReader(strings.NewReader(text))
	if err != nil {
		return nil, err
	}
	rules := make([]Rule, 0, len(parsed))
	for _, rule := range parsed {
		rules = append(rules, Rule{
			ID:      NewRuleID(),
			Action:  string(rule.Action),
			Type:    kindTypes[rule.Matcher.Kind],
			Value:   rule.Matcher.Value,
			Enabled: true,
		})
	}
	return rules, nil
}

// FormatRulesText writes rules in the CLI rules file format. Disabled rules
// are written as comments so that they survive a round trip unchanged in
// meaning.
func FormatRulesText(rules []Rule) string {
	var builder strings.Builder
	builder.WriteString("# Harp routing rules: <action> <matcher>\n")
	builder.WriteString("# Use with: harp proxy --rules <file>\n")
	for _, rule := range rules {
		if rule.Note != "" {
			fmt.Fprintf(&builder, "# %s\n", strings.ReplaceAll(rule.Note, "\n", " "))
		}
		prefix := ""
		if !rule.Enabled {
			prefix = "# "
		}
		fmt.Fprintf(&builder, "%s%s %s\n", prefix, rule.Action, rule.Matcher())
	}
	return builder.String()
}

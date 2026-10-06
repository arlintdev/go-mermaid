package mermaid

import (
	"regexp"
	"strings"

	"github.com/arlintdev/go-mermaid/internal/cssval"
	"github.com/arlintdev/go-mermaid/internal/theme"
)

var (
	initDirective = regexp.MustCompile(`(?s)%%\{\s*init(?:ialize)?\s*:(.*?)\}%%`)
	initTheme     = regexp.MustCompile(`["']theme["']\s*:\s*["']([A-Za-z]+)["']`)
	initVar       = regexp.MustCompile(`["']([A-Za-z]+)["']\s*:\s*["']([^"']*)["']`)
)

// directiveTheme reads the theme and the few theme variables a flowchart
// uses from a %%{init: ...}%% directive. Only a built-in theme name is
// taken, and every colour is checked by cssval; anything else is ignored.
func directiveTheme(src string) (name string, vars theme.Palette) {
	m := initDirective.FindStringSubmatch(src)
	if m == nil {
		return "", vars
	}
	body := m[1]
	if t := initTheme.FindStringSubmatch(body); t != nil {
		for _, n := range theme.Names() {
			if strings.EqualFold(n, t[1]) {
				name = n
			}
		}
	}
	for _, kv := range initVar.FindAllStringSubmatch(body, -1) {
		c, ok := cssval.Color(kv[2])
		if !ok {
			continue
		}
		switch kv[1] {
		case "primaryColor", "mainBkg":
			vars.NodeFill = c
		case "primaryBorderColor", "nodeBorder":
			vars.NodeStroke = c
		case "primaryTextColor", "textColor":
			vars.Text = c
		case "lineColor":
			vars.Edge = c
		case "clusterBkg":
			vars.ClusterFill = c
		case "clusterBorder":
			vars.ClusterStroke = c
		case "edgeLabelBackground":
			vars.LabelBackground = c
		}
	}
	return name, vars
}

package render

import (
	"fmt"
	"strings"

	"github.com/zboralski/spidermonkey-dumper/sm33/callgraph"
)

// DOT renders the callgraph in Graphviz DOT format.
// Style: NASA/Bauhaus — geometric, monochrome, thin rules, sparse color.
func DOT(g *callgraph.Graph, title string) string {
	const (
		nasaBlue = "#0B3D91"
		nasaRed  = "#FC3D21"
		black    = "#1A1A1A"
		gray     = "#9E9E9E"
		lightBg  = "#F5F5F5"
	)

	var b strings.Builder
	b.WriteString("digraph callgraph {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  splines=true;\n")
	b.WriteString("  nodesep=0.4;\n")
	b.WriteString("  ranksep=0.6;\n")
	fmt.Fprintf(&b, "  bgcolor=%q;\n", lightBg)
	fmt.Fprintf(&b, "  node [shape=rect, style=filled, fillcolor=white, color=%q, penwidth=0.5, fontname=\"Helvetica Neue,Helvetica,Arial\", fontsize=9, fontcolor=%q, height=0.3, margin=\"0.12,0.06\"];\n", black, black)
	fmt.Fprintf(&b, "  edge [color=%q, penwidth=0.5, arrowsize=0.5, arrowhead=vee];\n", gray)
	if title != "" {
		fmt.Fprintf(&b, "  labelloc=t;\n  labeljust=l;\n")
		fmt.Fprintf(&b, "  label=<<font face=\"Helvetica Neue,Helvetica\" point-size=\"8\" color=\"%s\">%s</font>>;\n", black, dotEscape(title))
	}
	b.WriteByte('\n')

	innerFuncs := map[string]bool{}
	for _, n := range g.Nodes {
		innerFuncs[n] = true
	}

	externalSeen := map[string]bool{}

	for _, n := range g.Nodes {
		id := dotID(n)
		switch {
		case n == "main":
			fmt.Fprintf(&b, "  %s [label=%q, fillcolor=%q, fontcolor=white, penwidth=0];\n", id, n, nasaBlue)
		case strings.HasPrefix(n, "anon#"):
			fmt.Fprintf(&b, "  %s [label=%q, style=\"filled,dashed\", color=%q, fontcolor=%q];\n", id, n, gray, gray)
		default:
			fmt.Fprintf(&b, "  %s [label=%q];\n", id, n)
		}
	}
	b.WriteByte('\n')

	for _, e := range g.Edges {
		callerID := dotID(e.Caller)
		calleeID := dotID(e.Callee)
		labelAttr := formatEdgeLabel(e.Args)
		if innerFuncs[e.Callee] {
			if labelAttr == "" {
				fmt.Fprintf(&b, "  %s -> %s;\n", callerID, calleeID)
			} else {
				fmt.Fprintf(&b, "  %s -> %s [%s];\n", callerID, calleeID, labelAttr[2:]) // strip leading ", "
			}
		} else {
			if !externalSeen[e.Callee] {
				externalSeen[e.Callee] = true
				if isAllCaps(e.Callee) {
					fmt.Fprintf(&b, "  %s [label=%q, shape=plaintext, style=\"\", fillcolor=none, fontname=\"Courier,monospace\", fontcolor=%q, fontsize=7];\n", calleeID, e.Callee, gray)
				} else {
					fmt.Fprintf(&b, "  %s [label=%q, shape=plaintext, style=\"\", fillcolor=none, fontcolor=%q, fontsize=8];\n", calleeID, e.Callee, nasaRed)
				}
			}
			if isAllCaps(e.Callee) {
				fmt.Fprintf(&b, "  %s -> %s [color=%q, style=dotted, penwidth=0.3%s];\n", callerID, calleeID, gray, labelAttr)
			} else {
				fmt.Fprintf(&b, "  %s -> %s [color=%q, style=dashed, penwidth=0.4%s];\n", callerID, calleeID, nasaRed, labelAttr)
			}
		}
	}

	b.WriteString("}\n")
	return b.String()
}

// formatEdgeLabel returns a DOT label attribute fragment for edge args.
// Returns "" if no args, or `, label=<...>` with per-arg coloring.
func formatEdgeLabel(args []string) string {
	if len(args) == 0 {
		return ""
	}
	const (
		teal       = "#00695C"
		deepOrange = "#D84315"
		nasaBlue   = "#0B3D91"
	)
	var b strings.Builder
	b.WriteString(`, label=<`)
	fmt.Fprintf(&b, `<font face="Helvetica Neue,Helvetica" point-size="7" color="%s"> (`, teal)
	for i, arg := range args {
		if i > 0 {
			b.WriteString(", ")
		}
		if strings.HasPrefix(arg, `"`) {
			fmt.Fprintf(&b, `<font color="%s">%s</font>`, deepOrange, dotEscape(arg))
		} else if arg == "true" || arg == "false" || arg == "null" {
			fmt.Fprintf(&b, `<font color="%s">%s</font>`, nasaBlue, arg)
		} else {
			b.WriteString(dotEscape(arg))
		}
	}
	b.WriteString(")</font>>")
	return b.String()
}

package compiler

import (
	"fmt"
	"regexp"
	"slices"
)

// prismVersion is prism_wasm.go's release; the gem's Ruby must match, as serialize.rb rejects other versions' bytes (decision 427).
const prismVersion = "1.9.0"

var prismGemDir = regexp.MustCompile(`/gems/prism-(\d[^/]*)/`)

// checkPrismGem fails at compile time what would be serialize.rb's "Invalid serialization" at the first parse (decision 427).
func checkPrismGem(names, dirs []string) error {
	for _, d := range dirs {
		if m := prismGemDir.FindStringSubmatch(d + "/"); m != nil && m[1] != prismVersion {
			return fmt.Errorf(`require "prism": the prism gem found is %s, but rugaroo's prism.wasm is %s; install it (gem install prism -v %s) or pin it in the Gemfile (decision 427)`, m[1], prismVersion, prismVersion)
		}
	}
	if slices.Contains(names, "prism") && !slices.ContainsFunc(dirs, prismGemDir.MatchString) {
		return fmt.Errorf(`require "prism" needs the prism %s gem installed (gem install prism -v %s): rugaroo compiles the gem's Ruby and runs its own prism.wasm under it (decision 427)`, prismVersion, prismVersion)
	}
	return nil
}

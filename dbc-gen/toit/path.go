// Copyright (C) 2021 Toitware ApS. All rights reserved.

package toit

import (
	"path"
	"strings"
)

func Path(p string) string {
	p = path.Clean(p)
	if !path.IsAbs(p) {
		p = "." + strings.ReplaceAll(p, "../", ".")
	} else {
		p = p[1:]
	}

	if path.Ext(p) == ".toit" {
		p = strings.TrimSuffix(p, ".toit")
	}

	return strings.ReplaceAll(p, "/", ".")
}

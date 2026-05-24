package web

import "html/template"

func weeklyFuncMap() template.FuncMap {
	return template.FuncMap{
		"carryBarBucket": func(pct int) int {
			if pct < 0 {
				pct = 0
			}
			if pct > 100 {
				pct = 100
			}
			return bucketTo5(pct)
		},
	}
}
